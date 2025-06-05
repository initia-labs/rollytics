package cmd

import (
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/initia-labs/rollytics/indexer/collector"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/scraper"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/rs/zerolog"
	slogzerolog "github.com/samber/slog-zerolog"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func IndexerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "indexer",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, cfgErr := config.GetConfig()
			if cfgErr != nil {
				panic(cfgErr)
			}

			zerologLogger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
			logger := slog.New(slogzerolog.Option{Level: cfg.GetLogLevel(), Logger: &zerologLogger}.NewZerologHandler())

			indexer, err := newIndexer(cfg, logger)
			if err != nil {
				panic(err)
			}

			indexer.Run()
		},
	}

	return cmd
}

type Indexer struct {
	height      int64
	cfg         *config.Config
	logger      *slog.Logger
	db          *orm.Database
	scraper     *scraper.Scraper
	collector   *collector.Collector
	blockMap    map[int64]indexertypes.ScrapedBlock
	blockChan   chan indexertypes.ScrapedBlock
	controlChan chan string
	paused      bool
	mtx         sync.Mutex
}

func newIndexer(cfg *config.Config, logger *slog.Logger) (*Indexer, error) {
	db, err := orm.OpenDB(cfg.GetDBConfig(), logger)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(); err != nil {
		return nil, err
	}

	var lastBlock types.CollectedBlock
	res := db.Where("chain_id = ?", cfg.GetChainConfig().ChainId).Order("height desc").Limit(1).Take(&lastBlock)
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		logger.Error("failed to get the last block from db", slog.Any("error", res.Error))
		return nil, errors.New("failed to get the last block from db")
	}

	return &Indexer{
		height:      lastBlock.Height + 1,
		cfg:         cfg,
		logger:      logger,
		db:          db,
		scraper:     scraper.New(cfg, logger),
		collector:   collector.New(logger, db, cfg),
		blockMap:    make(map[int64]indexertypes.ScrapedBlock),
		blockChan:   make(chan indexertypes.ScrapedBlock),
		controlChan: make(chan string),
	}, nil
}

func (i *Indexer) Run() {
	go i.scrape()
	go i.prepare()
	i.collect()
}

func (i *Indexer) scrape() {
	i.scraper.Run(i.height, i.blockChan, i.controlChan)
}

func (i *Indexer) prepare() {
	for block := range i.blockChan {
		if block.Height < i.height {
			continue
		}

		b := block
		go func() {
			if err := i.collector.Prepare(b); err != nil {
				panic(err)
			}

			i.mtx.Lock()
			i.blockMap[b.Height] = b
			i.mtx.Unlock()
		}()
	}
}

func (i *Indexer) collect() {
	for {
		i.mtx.Lock()

		switch {
		case len(i.blockMap) > 100 && !i.paused:
			i.controlChan <- "stop"
			i.paused = true
		case len(i.blockMap) < 50 && i.paused:
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

		if err := i.collector.Collect(block); err != nil {
			panic(err)
		}

		i.height++
	}
}
