package cmd

import (
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/initia-labs/rollytics/indexer/collector"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/scrapper"
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
	height    int64
	cfg       *config.Config
	logger    *slog.Logger
	db        *orm.Database
	scrapper  *scrapper.Scrapper
	collector *collector.Collector
	blockMap  map[int64]indexertypes.ScrappedBlock
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
		height:    lastBlock.Height + 1,
		cfg:       cfg,
		logger:    logger,
		db:        db,
		scrapper:  scrapper.New(cfg, logger),
		collector: collector.New(logger, db, cfg.GetChainConfig()),
		blockMap:  make(map[int64]indexertypes.ScrappedBlock),
	}, nil
}

func (i *Indexer) Run() {
	var (
		blockChan   = make(chan indexertypes.ScrappedBlock)
		controlChan = make(chan string)
		mtx         sync.Mutex
	)

	go i.scrapper.Run(i.height, blockChan, controlChan)

	go func() {
		for block := range blockChan {
			if block.Height < i.height {
				continue
			}

			b := block
			go func() {
				if err := i.collector.Prepare(b); err != nil {
					panic(err)
				}

				mtx.Lock()
				i.blockMap[b.Height] = b
				mtx.Unlock()
			}()
		}
	}()

	paused := false
	for {
		mtx.Lock()

		if len(i.blockMap) > 100 && !paused {
			controlChan <- "stop"
			paused = true
		} else if len(i.blockMap) < 50 && paused {
			controlChan <- "start"
			paused = false
		}

		block, ok := i.blockMap[i.height]
		if !ok {
			time.Sleep(i.cfg.GetCoolingDuration())
			mtx.Unlock()
			continue
		}

		if err := i.collector.Run(block); err != nil {
			panic(err)
		}

		delete(i.blockMap, i.height)
		i.height++
		mtx.Unlock()
	}
}
