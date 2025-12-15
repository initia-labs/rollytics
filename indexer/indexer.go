package indexer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/collector"
	"github.com/initia-labs/rollytics/indexer/extension"
	"github.com/initia-labs/rollytics/indexer/scraper"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/querier"
)

const (
	// ShutdownTimeout defines the maximum time to wait for graceful shutdown
	ShutdownTimeout = 30 * time.Second
)

type Indexer struct {
	cfg              *config.Config
	logger           *slog.Logger
	db               *orm.Database
	scraper          *scraper.Scraper
	collector        *collector.Collector
	extensionManager *extension.ExtensionManager
	blockMap         map[int64]indexertypes.ScrapedBlock
	blockChan        chan indexertypes.ScrapedBlock
	controlChan      chan string
	paused           bool
	mtx              sync.Mutex
	height           int64
	prepareCount     int
	ctx              context.Context
	querier          *querier.Querier
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Indexer {
	return &Indexer{
		cfg:              cfg,
		logger:           logger,
		db:               db,
		scraper:          scraper.New(cfg, logger),
		collector:        collector.New(cfg, logger, db),
		extensionManager: extension.New(cfg, logger, db),
		blockMap:         make(map[int64]indexertypes.ScrapedBlock),
		blockChan:        make(chan indexertypes.ScrapedBlock),
		controlChan:      make(chan string),
		querier:          querier.NewQuerier(cfg.GetChainConfig()),
	}
}

func (i *Indexer) Run(ctx context.Context) error {
	i.ctx = ctx
	// wait for the chain to be ready
	i.wait()

	var lastBlock types.CollectedBlock
	if err := i.db.
		Where("chain_id = ?", i.cfg.GetChainId()).
		Order("height desc").
		Limit(1).
		First(&lastBlock).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		i.logger.Error("failed to get the last block from db", slog.Any("error", err))
		return types.NewDatabaseError("get last block", err)
	}
	dbNext := lastBlock.Height + 1

	// get the current chain height for validation/clamping
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)
	chainHeight, err := i.querier.GetLatestHeight(ctx)
	if err != nil {
		i.logger.Error("failed to get chain height", slog.Any("error", err))
		return err
	}

	// determine the desired start height based on configuration and current DB state
	i.height = computeStartHeight(dbNext, chainHeight, i.cfg.StartHeightSet(), i.cfg.GetStartHeight())

	if dbNext > chainHeight {
		i.logger.Warn("database is ahead of chain",
			slog.Int64("db_height", dbNext-1),
			slog.Int64("chain_height", chainHeight),
		)
	}

	i.logger.Info("starting indexer",
		slog.Int64("db_resume_height", dbNext),
		slog.Int64("chain_height", chainHeight),
		slog.Bool("start_height_set", i.cfg.StartHeightSet()),
		slog.Int64("configured_start_height", i.cfg.GetStartHeight()),
		slog.Int64("effective_start_height", i.height),
	)

	// Use a wait group to track all goroutines
	var wg sync.WaitGroup

	// Start all components
	wg.Add(4)
	go func() {
		defer wg.Done()
		i.extend()
	}()
	go func() {
		defer wg.Done()
		i.scrape()
	}()
	go func() {
		defer wg.Done()
		i.prepare()
	}()
	go func() {
		defer wg.Done()
		i.collect()
	}()

	// Wait for context cancellation
	<-ctx.Done()
	i.logger.Info("indexer shutdown initiated")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	// Channel to signal completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		i.logger.Info("indexer shutdown completed gracefully")
	case <-shutdownCtx.Done():
		i.logger.Warn("indexer shutdown timed out, some goroutines may still be running")
	}

	return nil
}

func (i *Indexer) wait() {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)
	for {
		chainHeight, err := i.querier.GetLatestHeight(i.ctx)
		if err != nil {
			i.logger.Error("failed to get chain height", slog.Any("error", err))
			time.Sleep(5 * time.Second)
			continue
		}
		if chainHeight > types.MinChainHeightToStart {
			break
		}
		time.Sleep(types.ChainCheckInterval)
	}
}

func (i *Indexer) extend() {
	i.extensionManager.Run(i.ctx)
}

func (i *Indexer) scrape() {
	i.scraper.Run(i.ctx, i.height, i.blockChan, i.controlChan)
}

func (i *Indexer) prepare() {
	for {
		select {
		case <-i.ctx.Done():
			i.logger.Info("prepare() shutting down gracefully")
			return
		case block := <-i.blockChan:
			if block.Height < i.height {
				continue
			}

			i.mtx.Lock()
			i.prepareCount++
			i.mtx.Unlock()

			b := block
			go func() {
				defer func() {
					if r := recover(); r != nil {
						metrics.TrackPanic("indexer")
						panic(r) // re-panic
					}
				}()

				start := time.Now()
				indexerMetrics := metrics.GetMetrics().IndexerMetrics()

				// Retry prepare operation indefinitely for timeouts, only stop on context cancellation (Ctrl+C)
				for {
					// Check if context is cancelled (Ctrl+C) - this is the only way to stop
					select {
					case <-i.ctx.Done():
						i.logger.Info("prepare cancelled, stopping", slog.Int64("height", b.Height))
						i.decrementPrepareCount()
						return
					default:
					}

					err := i.collector.Prepare(i.ctx, b)
					if err == nil {
						// Success - break out of retry loop
						indexerMetrics.BlockProcessingTime.WithLabelValues("prepare").Observe(time.Since(start).Seconds())
						break
					}

					// Handle context cancellation (Ctrl+C) - stop immediately
					if errors.Is(err, context.Canceled) {
						i.logger.Info("prepare cancelled, stopping", slog.Int64("height", b.Height))
						i.decrementPrepareCount()
						return
					}

					// Handle timeout - retry indefinitely
					if errors.Is(err, context.DeadlineExceeded) {
						i.logger.Warn("prepare timed out, retrying", slog.Int64("height", b.Height))
						indexerMetrics.ProcessingErrors.WithLabelValues("prepare", "timeout_retry").Inc()
						metrics.TrackError("indexer", "prepare_timeout_retry")
						// Wait a bit before retrying
						select {
						case <-i.ctx.Done():
							i.logger.Info("prepare cancelled during retry wait, stopping", slog.Int64("height", b.Height))
							i.decrementPrepareCount()
							return
						case <-time.After(i.cfg.GetCoolingDuration()):
							// Continue retry loop
						}
						continue
					}

					// For other errors, log and panic (unchanged behavior)
					i.logger.Error("failed to prepare block", slog.Int64("height", b.Height), slog.Any("error", err))
					indexerMetrics.ProcessingErrors.WithLabelValues("prepare", "collector_error").Inc()
					metrics.TrackError("indexer", "prepare_error")
					panic(err)
				}

				// Success case: add to blockMap and decrement prepareCount after breaking from retry loop
				i.mtx.Lock()
				i.blockMap[b.Height] = b
				i.prepareCount--
				i.mtx.Unlock()
			}()
		}
	}
}

// decrementPrepareCount safely decrements the prepareCount counter
func (i *Indexer) decrementPrepareCount() {
	i.mtx.Lock()
	i.prepareCount--
	i.mtx.Unlock()
}

func (i *Indexer) collect() {
	for {
		select {
		case <-i.ctx.Done():
			i.logger.Info("collect() shutting down gracefully")
			return
		default:
		}

		i.mtx.Lock()

		inflightCount := len(i.blockMap) + i.prepareCount
		indexerMetrics := metrics.GetMetrics().IndexerMetrics()
		indexerMetrics.InflightBlocksCount.Set(float64(inflightCount))

		switch {
		case inflightCount > types.MaxInflightBlocks && !i.paused:
			i.controlChan <- "pause"
			i.paused = true
		case inflightCount < types.MinInflightBlocks && i.paused:
			i.controlChan <- "start"
			i.paused = false
		}

		block, ok := i.blockMap[i.height]
		delete(i.blockMap, i.height)
		i.mtx.Unlock()

		if !ok {
			select {
			case <-i.ctx.Done():
				i.logger.Info("collect() shutting down gracefully")
				return
			case <-time.After(i.cfg.GetCoolingDuration()):
			}
			continue
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					metrics.TrackPanic("indexer")
					panic(r) // re-panic
				}
			}()

			start := time.Now()
			indexerMetrics := metrics.GetMetrics().IndexerMetrics()
			if err := i.collector.Collect(block); err != nil {
				i.logger.Error("failed to collect block", slog.Int64("height", block.Height))
				indexerMetrics.ProcessingErrors.WithLabelValues("collect", "collector_error").Inc()
				metrics.TrackError("indexer", "collect_error")
				panic(err)
			}
			indexerMetrics.BlockProcessingTime.WithLabelValues("collect").Observe(time.Since(start).Seconds())

			indexerMetrics.BlocksProcessedTotal.Inc()
			indexerMetrics.CurrentBlockHeight.Set(float64(block.Height))
		}()

		i.height++
	}
}

// computeStartHeight returns the effective starting height based on:
// - the next height after the last block in DB (dbNext)
// - the current chain head (chainHead)
// - whether a start height is configured (startSet)
// - the configured numeric start value (startVal)
// Invariants: effective >= dbNext and effective <= chainHead.
func computeStartHeight(dbNext, chainHead int64, startSet bool, startVal int64) int64 {
	if !startSet {
		// Resume from DB next height when not explicitly set
		return dbNext
	}
	// Clamp in one expression using Go's built-in min/max (Go 1.21+):
	// lower bound 0, then at least dbNext, and at most chainHead.
	return max(dbNext, min(chainHead, max(int64(0), startVal)))
}
