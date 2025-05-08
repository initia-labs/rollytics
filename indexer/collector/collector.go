package collector

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/initia-labs/rollytics/indexer/collector/block"
	"github.com/initia-labs/rollytics/indexer/collector/nft"
	"github.com/initia-labs/rollytics/indexer/collector/tx"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type Collector struct {
	logger     *slog.Logger
	db         *orm.Database
	submodules map[string]types.Submodule
	mtx        sync.Mutex
}

func New(logger *slog.Logger, db *orm.Database, chainConfig *config.ChainConfig) *Collector {
	return &Collector{
		logger: logger.With("module", "collector"),
		db:     db,
		submodules: map[string]types.Submodule{
			block.SubmoduleName: block.New(logger, txConfig),
			tx.SubmoduleName:    tx.New(logger, txConfig),
			nft.SubmoduleName:   nft.New(logger, chainConfig),
		},
	}
}

func (c *Collector) Prepare(block types.ScrappedBlock) (err error) {
	var g errgroup.Group

	for _, sub := range c.submodules {
		s := sub
		g.Go(func() error {
			if err := s.Prepare(block); err != nil {
				c.logger.Error("failed to prepare data", slog.Int64("height", block.Height), slog.Any("error", err))
				return err
			}

			return nil
		})
	}

	return g.Wait()
}

func (c *Collector) Run(block types.ScrappedBlock) (err error) {
	tx := c.db.Begin()
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			if err = tx.Commit().Error; err != nil {
				c.logger.Error("failed to commit db transaction while handling commit", slog.Any("error", err))
			}
		}
	}()

	return tx.Transaction(func(tx *gorm.DB) error {
		for _, sub := range c.submodules {
			if err := sub.Collect(block, tx); err != nil {
				c.logger.Error(fmt.Sprintf("failed to collect %s", sub.Name()), slog.Int64("height", block.Height), slog.Any("error", err))
				return err
			}
		}

		return nil
	})
}
