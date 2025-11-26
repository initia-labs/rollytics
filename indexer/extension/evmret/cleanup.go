package evmret

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// TxDataWithEvents represents the structure of tx.data field containing events
type TxDataWithEvents struct {
	Events []abci.Event `json:"events"`
}

// FindRetOnlyAddresses parses Cosmos TX data and finds addresses that appear ONLY in ret attributes
func FindRetOnlyAddresses(txData json.RawMessage) ([]string, error) {
	var txDataWithEvents TxDataWithEvents
	if err := json.Unmarshal(txData, &txDataWithEvents); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tx data: %w", err)
	}

	retAddresses := make(map[string]struct{})
	nonRetAddresses := make(map[string]struct{})

	for _, event := range txDataWithEvents.Events {
		for _, attr := range event.Attributes {
			var addrs []string

			switch {
			case event.Type == evmtypes.EventTypeEVM && attr.Key == evmtypes.AttributeKeyLog:
				var log evmtypes.Log
				if err := json.Unmarshal([]byte(attr.Value), &log); err != nil {
					return nil, fmt.Errorf("failed to unmarshal evm log: %w", err)
				}

				addrs = append(addrs, extractAddressesFromValue(log.Address)...)
				for idx, topic := range log.Topics {
					if idx > 0 && strings.HasPrefix(topic, "0x000000000000000000000000") {
						addrs = append(addrs, extractAddressesFromValue(topic)...)
					}
				}
			default:
				addrs = append(addrs, extractAddressesFromValue(attr.Value)...)
			}

			if event.Type == evmtypes.EventTypeCall && attr.Key == evmtypes.AttributeKeyRet {
				for _, addr := range addrs {
					retAddresses[addr] = struct{}{}
				}
			} else {
				for _, addr := range addrs {
					nonRetAddresses[addr] = struct{}{}
				}
			}
		}
	}

	// Find ret-only addresses: addresses in ret but not in non-ret
	var retOnlyAddrs []string
	for addr := range retAddresses {
		if _, existsInNonRet := nonRetAddresses[addr]; !existsInNonRet {
			retOnlyAddrs = append(retOnlyAddrs, addr)
		}
	}

	return retOnlyAddrs, nil
}

// extractAddressesFromValue extracts valid EVM addresses from an attribute value
// It should produce the exact same results as grepAddressesFromTx for non-log attributes:
func extractAddressesFromValue(value string) []string {
	var addresses []string

	// Split on commas like grepAddressesFromTx does
	parts := strings.Split(value, ",")
	for _, part := range parts {
		token := strings.TrimSpace(part)

		acc, err := util.AccAddressFromString(token)
		if err != nil {
			continue
		}

		addr := util.BytesToHexWithPrefix(acc.Bytes())
		addresses = append(addresses, addr)
	}

	return addresses
}

// FilterNonSigners filters out account IDs that are signers for the given signerId
func FilterNonSigners(ctx context.Context, db *gorm.DB, accountIds []int64, signerId int64) ([]int64, error) {
	if len(accountIds) == 0 {
		return []int64{}, nil
	}

	// Filter out signers
	var nonSigners []int64
	for _, id := range accountIds {
		if id != signerId {
			nonSigners = append(nonSigners, id)
		}
	}

	return nonSigners, nil
}

// DeleteRetOnlyRecords deletes ret-only records from tx_accounts
func DeleteRetOnlyRecords(ctx context.Context, db *gorm.DB, accountIds []int64, sequence int64) (int64, error) {
	if len(accountIds) == 0 {
		return 0, nil
	}

	result := db.WithContext(ctx).
		Where("account_id IN ? AND sequence = ?", accountIds, sequence).
		Delete(&types.CollectedTxAccount{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete ret-only records: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// ProcessBatch processes a batch of transactions from startHeight to endHeight
func ProcessBatch(ctx context.Context, db *gorm.DB, logger *slog.Logger, startHeight, endHeight int64) (int64, error) {
	// Query transactions in the height range
	var txs []types.CollectedTx
	if err := db.WithContext(ctx).
		Where("height >= ? AND height <= ?", startHeight, endHeight).
		Order("height ASC").
		Find(&txs).Error; err != nil {
		return 0, fmt.Errorf("failed to query transactions: %w", err)
	}

	totalDeleted := int64(0)

	for _, tx := range txs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return totalDeleted, ctx.Err()
		default:
		}

		// Find ret-only addresses
		retOnlyAddrs, err := FindRetOnlyAddresses(tx.Data)
		if err != nil {
			// Convert hash bytes to hex string for logging
			hashStr := hex.EncodeToString(tx.Hash)
			logger.Warn("failed to parse tx data",
				slog.String("tx_hash", hashStr),
				slog.Int64("height", tx.Height),
				slog.Any("error", err))
			continue
		}

		if len(retOnlyAddrs) == 0 {
			continue
		}

		// Convert addresses to account IDs
		accountIds, err := util.GetOrCreateAccountIds(db, retOnlyAddrs, false)
		if err != nil {
			hashStr := hex.EncodeToString(tx.Hash)
			return totalDeleted, fmt.Errorf("failed to get account IDs for tx %s: %w", hashStr, err)
		}

		if len(accountIds) == 0 {
			continue
		}

		// Filter out signers
		nonSignerIds, err := FilterNonSigners(ctx, db, slices.Collect(maps.Values(accountIds)), tx.SignerId)
		if err != nil {
			hashStr := hex.EncodeToString(tx.Hash)
			return totalDeleted, fmt.Errorf("failed to filter signers for tx %s: %w", hashStr, err)
		}

		if len(nonSignerIds) == 0 {
			continue
		}

		// Delete ret-only records
		deleted, err := DeleteRetOnlyRecords(ctx, db, nonSignerIds, tx.Sequence)
		if err != nil {
			hashStr := hex.EncodeToString(tx.Hash)
			return totalDeleted, fmt.Errorf("failed to delete records for tx %s: %w", hashStr, err)
		}

		totalDeleted += deleted
	}

	return totalDeleted, nil
}
