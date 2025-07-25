package internal_tx

import (
	"errors"
	"log/slog"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

// Indexer is responsible for collecting and indexing internal transactions.
type Indexer struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Indexer {
	if cfg.GetVmType() != types.EVM && cfg.InternalTxEnabled() {
		return nil
	}

	return &Indexer{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}

func (i *Indexer) Run(height int64, commitChan chan int64) error {
	var lastItxs types.CollectedEvmInternalTx
	if err := i.db.Model(types.CollectedEvmInternalTx{}).Order("height desc").
		First(&lastItxs).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		i.logger.Error("failed to get the last block height: %w", slog.Any("error", err))
		return err
	}

	startHeight := lastItxs.Height + 1
	
	go func() {
		for h := startHeight; h <= height; h++ {
			commitChan <- h
		}
	}()

	i.collect(commitChan, startHeight)
	return nil
}
