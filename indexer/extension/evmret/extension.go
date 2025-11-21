package evmret

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
	"github.com/initia-labs/rollytics/types"
)

const (
	ExtensionName = "evm-ret-cleanup"
	BatchSize     = 1000 // Process 1000 blocks per batch
)

var _ exttypes.Extension = (*EvmRetCleanupExtension)(nil)

type EvmRetCleanupExtension struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database
}

// New creates a new EvmRetCleanupExtension instance
// Returns nil if the chain is not EVM-based or if the extension is disabled
func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *EvmRetCleanupExtension {
	if cfg.GetVmType() != types.EVM || !cfg.EvmRetCleanupEnabled() {
		return nil
	}

	return &EvmRetCleanupExtension{
		cfg:    cfg,
		logger: logger.With("extension", ExtensionName),
		db:     db,
	}
}

// Name returns the name of the extension
func (e *EvmRetCleanupExtension) Name() string {
	return ExtensionName
}

// Initialize ensures the status table exists and retrieves current status
func (e *EvmRetCleanupExtension) Initialize(ctx context.Context) (*types.CollectedEvmRetCleanupStatus, error) {
	// Create status table if it doesn't exist
	if err := e.db.WithContext(ctx).AutoMigrate(&types.CollectedEvmRetCleanupStatus{}); err != nil {
		return nil, fmt.Errorf("failed to create status table: %w", err)
	}

	// Try to retrieve existing status
	var status types.CollectedEvmRetCleanupStatus
	err := e.db.WithContext(ctx).First(&status).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create initial status record
			status = types.CollectedEvmRetCleanupStatus{
				LastCleanedHeight: 0,
				CorrectedRecords:  0,
			}
			if err := e.db.WithContext(ctx).Create(&status).Error; err != nil {
				return nil, fmt.Errorf("failed to create initial status: %w", err)
			}
			e.logger.Info("Initialized cleanup status")
			return &status, nil
		}
		return nil, fmt.Errorf("failed to retrieve cleanup status: %w", err)
	}

	return &status, nil
}

// Run executes the cleanup extension continuously
func (e *EvmRetCleanupExtension) Run(ctx context.Context) error {
	// Initialize and get status
	status, err := e.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	currentHeight := status.LastCleanedHeight + 1
	e.logger.Info("starting EVM ret cleanup",
		slog.Int64("start_height", currentHeight))

	// Continuous processing loop
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			e.logger.Info("cleanup stopped",
				slog.Int64("last_cleaned_height", status.LastCleanedHeight),
				slog.Int64("corrected_records", status.CorrectedRecords))
			return ctx.Err()
		default:
		}

		// Get the maximum height from the tx table
		var maxHeight int64
		if err := e.db.WithContext(ctx).
			Model(&types.CollectedTx{}).
			Select("COALESCE(MAX(height), 0)").
			Scan(&maxHeight).Error; err != nil {
			return fmt.Errorf("failed to get max height: %w", err)
		}

		// Wait if we've caught up to the current max height
		if currentHeight > maxHeight {
			// Small, context-aware backoff to avoid busy-polling the DB
			select {
			case <-ctx.Done():
				e.logger.Info("cleanup stopped",
					slog.Int64("last_cleaned_height", status.LastCleanedHeight),
					slog.Int64("corrected_records", status.CorrectedRecords))
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
			continue
		}

		endHeight := currentHeight + BatchSize - 1
		if endHeight > maxHeight {
			endHeight = maxHeight
		}

		// Process batch
		deleted, err := ProcessBatch(ctx, e.db.DB, e.logger, currentHeight, endHeight)
		if err != nil {
			return fmt.Errorf("failed to process batch [%d-%d]: %w", currentHeight, endHeight, err)
		}

		// Update status
		status.LastCleanedHeight = endHeight
		status.CorrectedRecords += deleted

		if err := e.updateStatus(ctx, status); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}

		e.logger.Info("evm ret cleanup processed height",
			slog.Int64("height", endHeight))

		currentHeight = endHeight + 1
	}
}

// updateStatus updates the status in the database
func (e *EvmRetCleanupExtension) updateStatus(ctx context.Context, status *types.CollectedEvmRetCleanupStatus) error {
	return e.db.WithContext(ctx).
		Model(&types.CollectedEvmRetCleanupStatus{}).
		Where("1 = 1"). // Update the single row
		Updates(map[string]interface{}{
			"last_cleaned_height": status.LastCleanedHeight,
			"corrected_records":   status.CorrectedRecords,
		}).Error
}
