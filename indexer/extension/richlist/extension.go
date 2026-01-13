package rich_list

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	evmrichlist "github.com/initia-labs/rollytics/indexer/extension/richlist/evmrichlist"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/querier"
)

const ExtensionName = "rich-list"

var _ exttypes.Extension = (*RichListExtension)(nil)

type RichListExtension struct {
	cfg         *config.Config
	logger      *slog.Logger
	db          *orm.Database
	startHeight int64 // Last produced/queued height
	requireInit bool
	querier     *querier.Querier
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *RichListExtension {
	// TODO: remove EVM check after all implementation
	if cfg.GetVmType() == types.WasmVM || !cfg.GetRichListEnabled() {
		return nil
	}

	return &RichListExtension{
		cfg:     cfg,
		logger:  logger.With("extension", ExtensionName),
		db:      db,
		querier: querier.NewQuerier(cfg.GetChainConfig()),
	}
}

func (r *RichListExtension) Initialize(ctx context.Context) error {
	var lastHeight types.CollectedRichListStatus
	err := r.db.WithContext(ctx).
		Model(types.CollectedRichListStatus{}).Limit(1).First(&lastHeight).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If no richlist status found, use the latest block height
			var lastBlock types.CollectedBlock
			if err := r.db.WithContext(ctx).
				Where("chain_id = ?", r.cfg.GetChainId()).
				Order("height DESC").
				Limit(1).
				First(&lastBlock).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					r.logger.Warn("no blocks found in database, starting from height 1")
					r.startHeight = 1
					return nil
				}
				r.logger.Error("failed to get the latest block height", slog.Any("error", err))
				return err
			}
			r.logger.Info("no richlist status found, using latest block height", slog.Int64("height", lastBlock.Height))
			r.startHeight = lastBlock.Height
			r.requireInit = true
			return nil
		}
		r.logger.Error("failed to get the last richlist status", slog.Any("error", err))
		return err
	}

	r.startHeight = lastHeight.Height + 1
	return nil
}

func (r *RichListExtension) Run(ctx context.Context) error {
	err := r.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize rich list extension: %w", err)
	}

	moduleAccounts, err := r.querier.GetMinterBurnerModuleAccounts(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch module accounts: %w", err)
	}

	switch r.cfg.GetVmType() {
	case types.EVM:
		if err := evmrichlist.Run(ctx, r.cfg, r.logger, r.db, r.startHeight, moduleAccounts, r.requireInit); err != nil {
			return err
		}
	default:
		return fmt.Errorf("rich list not supported: %v", r.cfg.GetVmType())
	}

	r.logger.Info("rich list extension shut down gracefully")
	return nil
}

// Name returns the name of the extension
func (r *RichListExtension) Name() string {
	return ExtensionName
}
