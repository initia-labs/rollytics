package collector

import (
	"log/slog"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/initia-labs/rollytics/indexer/collector/block"
	"github.com/initia-labs/rollytics/indexer/collector/tx"
	"github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"gorm.io/gorm"
)

type Collector struct {
	logger   *slog.Logger
	db       *orm.Database
	txConfig client.TxConfig
	block    types.Submodule
	tx       types.Submodule
}

func New(logger *slog.Logger, db *orm.Database, txConfig client.TxConfig) *Collector {
	return &Collector{
		logger:   logger.With("module", "collector"),
		db:       db,
		txConfig: txConfig,
		block:    block.New(logger, txConfig),
		tx:       tx.New(logger, txConfig),
	}
}

func (c Collector) Run(block types.ScrappedBlock) (err error) {
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
		if err := c.block.Collect(block, tx); err != nil {
			c.logger.Error("failed to collect block", slog.Int64("height", block.Height), slog.Any("error", err))
			return err
		}

		if err := c.tx.Collect(block, tx); err != nil {
			c.logger.Error("failed to collect tx", slog.Int64("height", block.Height), slog.Any("error", err))
			return err
		}

		return nil
	})
}
