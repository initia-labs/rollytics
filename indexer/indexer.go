package indexer

import (
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

func (i *Indexer) Run() error {
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

	go i.extend()
	go i.scrape()
	go i.prepare()
	i.collect()

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
	i.extensionManager.Run()
}

func (i *Indexer) scrape() {
	i.scraper.Run(i.height, i.blockChan, i.controlChan)
}

func (i *Indexer) prepare() {
	for block := range i.blockChan {
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

func (i *Indexer) collect() {
	for {
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
			time.Sleep(i.cfg.GetCoolingDuration())
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
