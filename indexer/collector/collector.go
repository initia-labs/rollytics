package collector

import (
	"log/slog"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/collector/block"
	evm_nft "github.com/initia-labs/rollytics/indexer/collector/evm-nft"
	move_nft "github.com/initia-labs/rollytics/indexer/collector/move-nft"
	"github.com/initia-labs/rollytics/indexer/collector/tx"
	wasm_nft "github.com/initia-labs/rollytics/indexer/collector/wasm-nft"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

type Collector struct {
	logger     *slog.Logger
	db         *orm.Database
	submodules []indexertypes.Submodule
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Collector {
	cdc := getCodec()
	blockSubmodule := block.New(logger, cdc)
	txSubmodule := tx.New(logger, cfg, cdc)
	var nftSubmodule indexertypes.Submodule
	switch cfg.GetVmType() {
	case types.MoveVM:
		nftSubmodule = move_nft.New(logger, cfg)
	case types.WasmVM:
		nftSubmodule = wasm_nft.New(logger, cfg)
	case types.EVM:
		nftSubmodule = evm_nft.New(logger, cfg)
	}

	return &Collector{
		logger: logger.With("module", "collector"),
		db:     db,
		submodules: []indexertypes.Submodule{
			blockSubmodule,
			txSubmodule,
			nftSubmodule,
		},
	}
}

func (c *Collector) Prepare(block indexertypes.ScrapedBlock) (err error) {
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

func (c *Collector) Collect(block indexertypes.ScrapedBlock) (err error) {
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
