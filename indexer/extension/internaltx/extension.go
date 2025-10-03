package internaltx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/sentry_integration"
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

func (i *InternalTxExtension) Run(ctx context.Context) error {
	if err := CheckNodeVersion(i.cfg); err != nil {
		i.logger.Warn("skipping internal transaction indexing", slog.Any("reason", err.Error()))
		return nil
	}

	// Initialize last height with context
	var lastItx types.CollectedEvmInternalTx
	if err := i.db.WithContext(ctx).Model(types.CollectedEvmInternalTx{}).Order("height desc").
		First(&lastItx).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		i.logger.Error("failed to get the last block height", slog.Any("error", err))
		return err
	}
	i.lastHeight = lastItx.Height

	// Use ticker instead of sleep for cancellation support
	ticker := time.NewTicker(i.cfg.GetInternalTxConfig().GetPollInterval())
	defer ticker.Stop()

	// Initial run immediately
	if err := i.processBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
		i.logger.Error("initial batch processing failed", slog.Any("error", err))
	}

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("internal tx extension shutting down gracefully",
				slog.String("reason", ctx.Err().Error()))
			return nil

		case <-ticker.C:
			if err := i.processBatch(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					i.logger.Info("batch processing cancelled")
					return nil
				}
				// Log error but continue processing
				i.logger.Error("batch processing failed",
					slog.Any("error", err),
					slog.Int64("last_height", i.lastHeight))
				// Continue to next iteration instead of panicking
			}
		}
	}
}

// processBatch processes the next batch of blocks
func (i *InternalTxExtension) processBatch(ctx context.Context) error {
	transaction, ctx := sentry_integration.StartSentryTransaction(ctx, "processBatch", "Processing batch of internal transactions")
	defer transaction.Finish()
	var heights []int64

	// Check context before DB operation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Query blocks with context support
	if err := i.db.WithContext(ctx).
		Model(&types.CollectedBlock{}).
		Where("chain_id = ?", i.cfg.GetChainId()).
		Where("height > ?", i.lastHeight).
		Where("tx_count > 0").
		Order("height ASC").
		Limit(i.cfg.GetInternalTxConfig().GetBatchSize()).
		Pluck("height", &heights).Error; err != nil {
		return fmt.Errorf("failed to query blocks: %w", err)
	}

	if len(heights) == 0 {
		return nil // No new blocks to process
	}

	i.logger.Debug("processing batch",
		slog.Int("count", len(heights)),
		slog.Int64("from", heights[0]),
		slog.Int64("to", heights[len(heights)-1]))

	if err := i.collect(ctx, heights); err != nil {
		return fmt.Errorf("collect failed for heights %v: %w", heights, err)
	}

	i.lastHeight = heights[len(heights)-1]
	return nil
}

// Name returns the name of the extension
func (i *InternalTxExtension) Name() string {
	return ExtensionName
}
