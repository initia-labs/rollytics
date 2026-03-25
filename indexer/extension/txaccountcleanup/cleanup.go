package txaccountcleanup

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	abci "github.com/cometbft/cometbft/abci/types"
	"gorm.io/gorm"

	tx "github.com/initia-labs/rollytics/indexer/collector/tx"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
)

type txDataWithEvents struct {
	Events []abci.Event `json:"events"`
}

func ProcessBatch(ctx context.Context, db *gorm.DB, logger *slog.Logger, startSeq, endSeq int64) (lastProcessedSeq, totalDeleted, totalInserted int64, err error) {
	lastProcessedSeq = startSeq - 1

	// Query txs in sequence range
	var txs []types.CollectedTx
	if err := db.WithContext(ctx).
		Where("sequence >= ? AND sequence <= ?", startSeq, endSeq).
		Order("sequence ASC").
		Find(&txs).Error; err != nil {
		return lastProcessedSeq, 0, 0, fmt.Errorf("failed to query transactions: %w", err)
	}

	if len(txs) == 0 {
		return endSeq, 0, 0, nil
	}

	// Query actual tx_accounts in sequence range
	var actualEntries []types.CollectedTxAccount
	if err := db.WithContext(ctx).
		Where("sequence >= ? AND sequence <= ?", startSeq, endSeq).
		Find(&actualEntries).Error; err != nil {
		return lastProcessedSeq, 0, 0, fmt.Errorf("failed to query tx_accounts: %w", err)
	}

	// Group actual entries by sequence
	actualBySeq := make(map[int64][]types.CollectedTxAccount)
	for _, entry := range actualEntries {
		actualBySeq[entry.Sequence] = append(actualBySeq[entry.Sequence], entry)
	}

	for _, collectedTx := range txs {
		select {
		case <-ctx.Done():
			return lastProcessedSeq, totalDeleted, totalInserted, ctx.Err()
		default:
		}

		deleted, inserted, err := reconcileTx(ctx, db, logger, collectedTx, actualBySeq[collectedTx.Sequence])
		if err != nil {
			hashStr := hex.EncodeToString(collectedTx.Hash)
			return lastProcessedSeq, totalDeleted, totalInserted, fmt.Errorf("failed to reconcile tx %s at sequence %d: %w", hashStr, collectedTx.Sequence, err)
		}

		totalDeleted += deleted
		totalInserted += inserted
		lastProcessedSeq = collectedTx.Sequence
	}

	return endSeq, totalDeleted, totalInserted, nil
}

func reconcileTx(ctx context.Context, db *gorm.DB, logger *slog.Logger, collectedTx types.CollectedTx, actual []types.CollectedTxAccount) (deleted, inserted int64, err error) {
	ctxDB := db.WithContext(ctx)

	// Parse events from stored tx data
	var txData txDataWithEvents
	if err := json.Unmarshal(collectedTx.Data, &txData); err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal tx data: %w", err)
	}

	// Re-derive addresses from events
	addrs, err := tx.GrepAddressesFromTx(txData.Events, ctxDB)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to grep addresses: %w", err)
	}

	// Convert to account IDs (don't create new accounts)
	accountIdMap, err := cache.GetOrCreateAccountIds(ctxDB, addrs, false)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get account IDs: %w", err)
	}

	// Build expected set (filter out id=0)
	expectedSet := make(map[int64]struct{})
	for _, id := range accountIdMap {
		if id != 0 {
			expectedSet[id] = struct{}{}
		}
	}

	// Add signer (always expected)
	if collectedTx.SignerId != 0 {
		expectedSet[collectedTx.SignerId] = struct{}{}
	}

	// Build actual set
	actualSet := make(map[int64]types.CollectedTxAccount)
	for _, entry := range actual {
		actualSet[entry.AccountId] = entry
	}

	// Compute extras (in actual but not expected) → DELETE
	var extraIds []int64
	for accountId := range actualSet {
		if _, ok := expectedSet[accountId]; !ok {
			extraIds = append(extraIds, accountId)
		}
	}

	// Compute missing (in expected but not actual) → INSERT
	var missingEntries []types.CollectedTxAccount
	for accountId := range expectedSet {
		if _, ok := actualSet[accountId]; !ok {
			missingEntries = append(missingEntries, types.CollectedTxAccount{
				AccountId: accountId,
				Sequence:  collectedTx.Sequence,
				Signer:    accountId == collectedTx.SignerId,
			})
		}
	}

	// Compute stale signer flags (in both sets but wrong Signer value) → UPDATE
	var staleSigner []int64
	for accountId, entry := range actualSet {
		if _, ok := expectedSet[accountId]; !ok {
			continue
		}
		expectedSigner := accountId == collectedTx.SignerId
		if entry.Signer != expectedSigner {
			staleSigner = append(staleSigner, accountId)
		}
	}

	if len(extraIds) == 0 && len(missingEntries) == 0 && len(staleSigner) == 0 {
		return 0, 0, nil
	}

	hashStr := hex.EncodeToString(collectedTx.Hash)

	err = db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		// Delete extras
		if len(extraIds) > 0 {
			result := txDB.
				Where("account_id IN ? AND sequence = ?", extraIds, collectedTx.Sequence).
				Delete(&types.CollectedTxAccount{})
			if result.Error != nil {
				return fmt.Errorf("failed to delete extra entries: %w", result.Error)
			}
			deleted = result.RowsAffected

			logger.Info("deleted extra tx_accounts entries",
				slog.String("tx_hash", hashStr),
				slog.Int64("sequence", collectedTx.Sequence),
				slog.Int64("count", deleted))
		}

		// Insert missing
		if len(missingEntries) > 0 {
			result := txDB.Clauses(orm.DoNothingWhenConflict).Create(&missingEntries)
			if result.Error != nil {
				return fmt.Errorf("failed to insert missing entries: %w", result.Error)
			}
			inserted = result.RowsAffected

			logger.Info("inserted missing tx_accounts entries",
				slog.String("tx_hash", hashStr),
				slog.Int64("sequence", collectedTx.Sequence),
				slog.Int64("count", inserted))
		}

		// Fix stale signer flags
		if len(staleSigner) > 0 {
			for _, accountId := range staleSigner {
				expectedSigner := accountId == collectedTx.SignerId
				result := txDB.
					Model(&types.CollectedTxAccount{}).
					Where("account_id = ? AND sequence = ?", accountId, collectedTx.Sequence).
					Update("signer", expectedSigner)
				if result.Error != nil {
					return fmt.Errorf("failed to update signer flag: %w", result.Error)
				}
			}

			logger.Info("fixed stale signer flags",
				slog.String("tx_hash", hashStr),
				slog.Int64("sequence", collectedTx.Sequence),
				slog.Int("count", len(staleSigner)))
		}

		return nil
	})

	return deleted, inserted, err
}
