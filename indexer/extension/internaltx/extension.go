package internaltx

import (
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

const ExtensionName = "internal-tx"

var _ exttypes.Extension = (*InternalTxExtension)(nil)

// InternalTxExtension is responsible for collecting and indexing internal transactions.
type InternalTxExtension struct {
	cfg        *config.Config
	logger     *slog.Logger
	db         *orm.Database
	lastHeight int64
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *InternalTxExtension {
	if cfg.GetVmType() != types.EVM || !cfg.InternalTxEnabled() {
		return nil
	}

	return &InternalTxExtension{
		cfg:    cfg,
		logger: logger.With("extension", ExtensionName),
		db:     db,
	}
}

func (i *InternalTxExtension) Run() error {
	if err := CheckNodeVersion(i.cfg); err != nil {
		i.logger.Warn("skipping internal transaction indexing", slog.Any("reason", err.Error()))
		return nil
	}
	var lastItx types.CollectedEvmInternalTx
	if err := i.db.Model(types.CollectedEvmInternalTx{}).Order("height desc").
		First(&lastItx).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		i.logger.Error("failed to get the last block height: %w", slog.Any("error", err))
		return err
	}
	i.lastHeight = lastItx.Height

	for {
		start := time.Now()
		var heights []int64

		// Check the diff between the last indexed height and the current height
		if err := i.db.Model(&types.CollectedBlock{}).
			Where("chain_id = ?", i.cfg.GetChainId()).
			Where("height > ?", i.lastHeight).
			Where("tx_count > 0").
			Order("height ASC").
			Limit(i.cfg.GetInternalTxConfig().GetBatchSize()).Pluck("height", &heights).Error; err != nil {
			i.logger.Error("failed to get blocks to process", slog.Any("error", err))
			panic(err)
		}
		if len(heights) > 0 {
			if err := i.collect(heights); err != nil {
				return err
			}
			i.lastHeight = heights[len(heights)-1]
		}

		time.Sleep(i.cfg.GetInternalTxConfig().GetPollInterval())
		overall := time.Since(start)
		i.logger.Info("time to get blocks to process", slog.Float64("overall_sec", overall.Seconds()), slog.Int("count", len(heights)))
	}
}

// Name returns the name of the extension
func (i *InternalTxExtension) Name() string {
	return ExtensionName
}
