package internal_tx

import (
	"errors"
	"log/slog"
	"time"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

const (
	batchSize = 5
)

// Indexer is responsible for collecting and indexing internal transactions.
type Indexer struct {
	cfg         *config.Config
	logger      *slog.Logger
	db          *orm.Database
	lastIndexed int64
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Indexer {
	if cfg.GetVmType() != types.EVM && cfg.InternalTxEnabled() {
		return nil
	}

	return &Indexer{
		cfg:         cfg,
		logger:      logger,
		db:          db,
		lastIndexed: 0,
	}
}

func (i *Indexer) Run() error {
	var lastItxs types.CollectedEvmInternalTx
	if err := i.db.Model(types.CollectedEvmInternalTx{}).Order("height desc").
		First(&lastItxs).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		i.logger.Error("failed to get the last block height: %w", slog.Any("error", err))
		return err
	}
	i.lastIndexed = lastItxs.Height

	for {
		var heights []int64
		// Check the diff between the last indexed height and the current height
		if err := i.db.Model(&types.CollectedBlock{}).
			Where("chain_id = ?", i.cfg.GetChainId()).
			Where("height > ?", i.lastIndexed).
			Where("tx_count > 0").
			Order("height ASC").
			Limit(batchSize).Pluck("height", &heights).Error; err != nil {
			i.logger.Error("failed to get blocks to process", slog.Any("error", err))
			continue
		}

		if len(heights) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		i.collect(heights)

		if len(heights) > 0 {
			i.lastIndexed = heights[len(heights)-1]
		}

		time.Sleep(5 * time.Second)
	}
}
