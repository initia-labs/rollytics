package collector

import (
	"log/slog"

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
}

func New(logger *slog.Logger, db *orm.Database, cfg *config.Config) *Collector {
	return &Collector{
		logger: logger.With("module", "collector"),
		db:     db,
		submodules: map[string]types.Submodule{
			block.SubmoduleName: block.New(logger, txConfig),
			tx.SubmoduleName:    tx.New(logger, cfg, txConfig, cdc),
			nft.SubmoduleName:   nft.New(logger, cfg),
		},
	}
}

func (c *Collector) Prepare(block types.ScrappedBlock) (err error) {
	var g errgroup.Group

	for _, sub := range c.submodules {
		s := sub
		g.Go(func() error {
			return s.Prepare(block)
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	c.logger.Info("prepared data", slog.Int64("height", block.Height))
	return nil
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
				return err
			}
		}

		c.logger.Info("indexed block", slog.Int64("height", block.Height))
		return nil
	})
}
