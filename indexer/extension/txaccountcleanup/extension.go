package txaccountcleanup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

const (
	ExtensionName = "tx-account-cleanup"
	BatchSize     = 1000
)

var _ exttypes.Extension = (*TxAccountCleanupExtension)(nil)

type TxAccountCleanupExtension struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *TxAccountCleanupExtension {
	if !cfg.TxAccountCleanupEnabled() {
		return nil
	}

	return &TxAccountCleanupExtension{
		cfg:    cfg,
		logger: logger.With("extension", ExtensionName),
		db:     db,
	}
}

func (e *TxAccountCleanupExtension) Name() string {
	return ExtensionName
}

func (e *TxAccountCleanupExtension) Initialize(ctx context.Context) (*types.CollectedTxAccountCleanupStatus, error) {
	var status types.CollectedTxAccountCleanupStatus
	err := e.db.WithContext(ctx).First(&status).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Initialize with -1 to indicate no cleanup has been done yet;
			// Run() will set it to max(sequence) on first iteration.
			status = types.CollectedTxAccountCleanupStatus{
				LastCleanedSequence: -1,
				DeletedRecords:      0,
				InsertedRecords:     0,
			}
			if err := e.db.WithContext(ctx).Create(&status).Error; err != nil {
				return nil, fmt.Errorf("failed to create initial status: %w", err)
			}
			e.logger.Info("initialized cleanup status")
			return &status, nil
		}
		return nil, fmt.Errorf("failed to retrieve cleanup status: %w", err)
	}

	return &status, nil
}

func (e *TxAccountCleanupExtension) Run(ctx context.Context) error {
	status, err := e.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Determine starting point: scan from max sequence down to 0
	currentSeq := status.LastCleanedSequence
	if currentSeq < 0 {
		// First run: start from the current max sequence
		var maxSeq int64
		if err := e.db.WithContext(ctx).
			Model(&types.CollectedTx{}).
			Select("COALESCE(MAX(sequence), 0)").
			Scan(&maxSeq).Error; err != nil {
			return fmt.Errorf("failed to get max sequence: %w", err)
		}
		currentSeq = maxSeq
		status.LastCleanedSequence = currentSeq
		if err := e.updateStatus(ctx, status); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}

	e.logger.Info("starting tx account cleanup (reverse)",
		slog.Int64("start_sequence", currentSeq))

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("cleanup stopped",
				slog.Int64("last_cleaned_sequence", status.LastCleanedSequence),
				slog.Int64("deleted_records", status.DeletedRecords),
				slog.Int64("inserted_records", status.InsertedRecords))
			return ctx.Err()
		default:
		}

		if currentSeq <= 0 {
			e.logger.Info("cleanup complete",
				slog.Int64("deleted_records", status.DeletedRecords),
				slog.Int64("inserted_records", status.InsertedRecords))
			return nil
		}

		startSeq := max(currentSeq-BatchSize+1, 1)

		_, deleted, inserted, batchErr := ProcessBatch(ctx, e.db.DB, e.logger, startSeq, currentSeq)
		if batchErr != nil {
			return fmt.Errorf("failed to process batch [%d-%d]: %w", startSeq, currentSeq, batchErr)
		}

		status.LastCleanedSequence = startSeq - 1
		status.DeletedRecords += deleted
		status.InsertedRecords += inserted

		if err := e.updateStatus(ctx, status); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}

		if deleted > 0 || inserted > 0 {
			e.logger.Info("tx account cleanup processed batch",
				slog.Int64("start_sequence", startSeq),
				slog.Int64("end_sequence", currentSeq),
				slog.Int64("batch_deleted", deleted),
				slog.Int64("batch_inserted", inserted))
		}

		currentSeq = startSeq - 1
	}
}

func (e *TxAccountCleanupExtension) updateStatus(ctx context.Context, status *types.CollectedTxAccountCleanupStatus) error {
	return e.db.WithContext(ctx).
		Model(&types.CollectedTxAccountCleanupStatus{}).
		Where("1 = 1").
		Updates(map[string]any{
			"last_cleaned_sequence": status.LastCleanedSequence,
			"deleted_records":       status.DeletedRecords,
			"inserted_records":      status.InsertedRecords,
		}).Error
}
