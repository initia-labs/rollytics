package rich_list

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	evmrichlist "github.com/initia-labs/rollytics/indexer/extension/richlist/evmrichlist"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

const ExtensionName = "rich-list"

var _ exttypes.Extension = (*RichListExtension)(nil)

type RichListExtension struct {
	cfg         *config.Config
	logger      *slog.Logger
	db          *orm.Database
	startHeight int64 // Last produced/queued height
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *RichListExtension {
	if !cfg.GetRichListEnabled() {
		return nil
	}

	return &RichListExtension{
		cfg:    cfg,
		logger: logger.With("extension", ExtensionName),
		db:     db,
	}
}

func (r *RichListExtension) Initialize(ctx context.Context) error {
	var lastHeight types.CollectedRichListStatus
	if err := r.db.WithContext(ctx).
		Model(types.CollectedRichListStatus{}).Limit(1).First(&lastHeight).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		r.logger.Error("failed to get the last block height", slog.Any("error", err))
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

	moduleAccounts, err := richlistutils.FetchMinterBurnerModuleAccounts(ctx, r.cfg.GetChainConfig().RestUrl)
	if err != nil {
		return fmt.Errorf("failed to fetch module accounts: %w", err)
	}

	switch r.cfg.GetVmType() {
	case types.EVM:
		if err := evmrichlist.Run(ctx, r.cfg, r.logger, r.db, r.startHeight, moduleAccounts); err != nil {
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
