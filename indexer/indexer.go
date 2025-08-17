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
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
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
	i.height = lastBlock.Height + 1

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
		chainHeight, err := util.GetLatestHeight(client, i.cfg)
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
				if err := i.collector.Prepare(b); err != nil {
					i.logger.Error("failed to prepare block", slog.Int64("height", b.Height), slog.Any("error", err))
					indexerMetrics.ProcessingErrors.WithLabelValues("prepare", "collector_error").Inc()
					metrics.TrackError("indexer", "prepare_error")
					panic(err)
				}
				indexerMetrics.BlockProcessingTime.WithLabelValues("prepare").Observe(time.Since(start).Seconds())

				i.mtx.Lock()
				i.blockMap[b.Height] = b
				i.prepareCount--
				i.mtx.Unlock()
			}()
		}
	}
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
