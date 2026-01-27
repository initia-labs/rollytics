package rich_list

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/extension/richlist/scraper"

	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
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
	if !cfg.GetRichListEnabled() {
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
			var lastBlockHeight int64
			for {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if lastBlockHeight, err = richlistutils.GetLatestCollectedBlock(ctx, r.db.DB, r.cfg.GetChainId()); err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						r.logger.Warn("no blocks found in database, waiting for blocks to be indexed")
						select {
						case <-time.After(5 * time.Second):
							continue
						case <-ctx.Done():
							return ctx.Err()
						}
					}
					r.logger.Error("failed to get the latest block height", slog.Any("error", err))
					return err
				}
				break
			}

			r.logger.Info("no richlist status found, using latest block height", slog.Int64("height", lastBlockHeight))
			r.startHeight = lastBlockHeight
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

	richListProcessor, err := scraper.New(r.cfg, r.logger, r.db)
	if err != nil {
		return err
	}

	if err := richListProcessor.Run(ctx, r.startHeight, r.requireInit, moduleAccounts); err != nil {
		return err
	}

	r.logger.Info("rich list extension shut down gracefully")
	return nil
}

// Name returns the name of the extension
func (r *RichListExtension) Name() string {
	return ExtensionName
}
