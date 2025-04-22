package cmd

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/initia-labs/rollytics/indexer/collector"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/scrapper"
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
	Scrapper  *scrapper.Scrapper
	Collector *collector.Collector
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
		Scrapper:  scrapper.New(cfg, logger),
		Collector: collector.New(logger, db),
	}, nil
}

func (i Indexer) Run() {
	go i.Scrapper.Run(i.height)

	for {
		block, ok := i.Scrapper.GetBlock(i.height)
		if !ok {
			time.Sleep(i.cfg.GetCoolingDuration())
			continue
		}

		if err := i.Collector.Run(block); err != nil {
			i.logger.Error("failed to collect data for block", slog.Int64("height", i.height), slog.Any("error", err))
			panic(err)
		}

		i.logger.Info("indexed block", slog.Int64("height", i.height))
		i.Scrapper.DeleteBlock(i.height)
		i.height++
	}
}
