package internaltx

import (
	"errors"
	"log/slog"
	"time"

	"github.com/initia-labs/rollytics/config"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

var _ exttypes.Extension = (*InternalTxExtension)(nil)

// InternalTxExtension is responsible for collecting and indexing internal transactions.
type InternalTxExtension struct {
	cfg         *config.Config
	logger      *slog.Logger
	db          *orm.Database
	lastIndexed int64
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *InternalTxExtension {
	if cfg.GetVmType() != types.EVM || !cfg.InternalTxEnabled() {
		return nil
	}

	return &InternalTxExtension{
		cfg:         cfg,
		logger:      logger,
		db:          db,
		lastIndexed: 0,
	}
}

func (i *InternalTxExtension) Run() error {
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
			Limit(i.cfg.GetInternalTxConfig().GetBatchSize()).Pluck("height", &heights).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			i.logger.Error("failed to get blocks to process", slog.Any("error", err))
			panic(err)
		}

		if len(heights) == 0 {
			continue
		}

		if err := i.collect(heights); err != nil {
			panic(err)
		}

		i.lastIndexed = heights[len(heights)-1]

		time.Sleep(i.cfg.GetInternalTxConfig().GetPollInterval())

	}
}

// Name returns the name of the extension
func (i *InternalTxExtension) Name() string {
	return "internal-tx"
}
