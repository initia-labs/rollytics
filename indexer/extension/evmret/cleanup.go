package evmret

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
			addresses := extractAddressesFromValue(attr.Value)

			if attr.Key == "ret" {
				for _, addr := range addresses {
					retAddresses[addr] = struct{}{}
				}
			} else {
				for _, addr := range addresses {
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
// - Split the value by commas
// - Accept tokens matching anchored regex ^0x[0-9a-fA-F]{1,64}$
// - Normalize to a 20-byte address by taking the last 40 hex chars or left-padding
func extractAddressesFromValue(value string) []string {
	var addresses []string

	// Anchored regex used by historical indexer logic(findAllHexAddress in address.go)
	anchoredHex := regexp.MustCompile(`^0x(?:[a-fA-F0-9]{1,64})$`)

	// Split on commas like grepAddressesFromTx does
	parts := strings.Split(value, ",")
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if !anchoredHex.MatchString(token) {
			continue
		}

		// Drop 0x
		hexPart := token[2:]
		// Normalize to lowercase
		hexPart = strings.ToLower(hexPart)

		// Derive a 20-byte address from up to 64 hex chars
		if len(hexPart) >= 40 {
			hexPart = hexPart[len(hexPart)-40:]
		} else {
			hexPart = strings.Repeat("0", 40-len(hexPart)) + hexPart
		}

		addr := "0x" + hexPart
		if isValidEVMAddress(addr) {
			addresses = append(addresses, addr)
		}
	}

	return addresses
}

// isValidEVMAddress checks if a string is a valid EVM address
func isValidEVMAddress(addr string) bool {
	// Must start with 0x
	if !strings.HasPrefix(addr, "0x") {
		return false
	}

	// Remove 0x prefix
	hexPart := addr[2:]

	// Must be exactly 40 hex characters (20 bytes)
	if len(hexPart) != 40 {
		return false
	}

	// Must be valid hex
	_, err := hex.DecodeString(hexPart)
	return err == nil
}

// GetAccountIds converts addresses to account IDs using account_dict
func GetAccountIds(ctx context.Context, db *gorm.DB, addresses []string) ([]int64, error) {
	if len(addresses) == 0 {
		return []int64{}, nil
	}

	// Convert string addresses to bytes for comparison
	addressBytes := make([][]byte, len(addresses))
	for i, addr := range addresses {
		addrStr := strings.TrimPrefix(addr, "0x")
		b, err := hex.DecodeString(addrStr)
		if err != nil {
			return nil, fmt.Errorf("invalid hex address %q: %w", addr, err)
		}
		addressBytes[i] = b
	}

	var accounts []types.CollectedAccountDict
	if err := db.WithContext(ctx).
		Where("account IN ?", addressBytes).
		Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get account IDs: %w", err)
	}

	ids := make([]int64, 0, len(accounts))
	for _, acc := range accounts {
		ids = append(ids, acc.Id)
	}

	return ids, nil
}

// FilterNonSigners filters out account IDs that are signers for the given sequence
func FilterNonSigners(ctx context.Context, db *gorm.DB, accountIds []int64, sequence int64) ([]int64, error) {
	if len(accountIds) == 0 {
		return []int64{}, nil
	}

	// Get all accounts that are signers for this sequence
	var signerRecords []types.CollectedTxAccount
	if err := db.WithContext(ctx).
		Where("sequence = ? AND account_id IN ? AND signer = ?", sequence, accountIds, true).
		Find(&signerRecords).Error; err != nil {
		return nil, fmt.Errorf("failed to check signers: %w", err)
	}

	// Create a set of signer IDs
	signerSet := make(map[int64]struct{})
	for _, record := range signerRecords {
		signerSet[record.AccountId] = struct{}{}
	}

	// Filter out signers
	var nonSigners []int64
	for _, id := range accountIds {
		if _, isSigner := signerSet[id]; !isSigner {
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
		Order("height ASC, sequence ASC").
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
		accountIds, err := GetAccountIds(ctx, db, retOnlyAddrs)
		if err != nil {
			hashStr := hex.EncodeToString(tx.Hash)
			return totalDeleted, fmt.Errorf("failed to get account IDs for tx %s: %w", hashStr, err)
		}

		if len(accountIds) == 0 {
			continue
		}

		// Filter out signers
		nonSignerIds, err := FilterNonSigners(ctx, db, accountIds, tx.Sequence)
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

		if deleted > 0 {
			totalDeleted += deleted
			hashStr := hex.EncodeToString(tx.Hash)

			var accRows []types.CollectedAccountDict
			if err := db.WithContext(ctx).
				Where("id IN ?", nonSignerIds).
				Find(&accRows).Error; err != nil {
				logger.Warn("failed to load accounts for logging corrected entries",
					slog.Any("account_ids", nonSignerIds),
					slog.Any("error", err))
			}

			var hexAddrs []string
			var bech32Addrs []string
			for _, acc := range accRows {
				hexAddrs = append(hexAddrs, util.BytesToHexWithPrefix(acc.Account))
				// Convert to bech32 (uses global bech32 config/prefix)
				bech32Addrs = append(bech32Addrs, sdk.AccAddress(acc.Account).String())
			}

			logger.Info("corrected ret-only entries",
				slog.String("tx_hash", hashStr),
				slog.Int64("height", tx.Height),
				slog.Int64("sequence", tx.Sequence),
				slog.Any("account_ids", nonSignerIds),
				slog.Any("hex_addresses", hexAddrs),
				slog.Any("bech32_addresses", bech32Addrs),
				slog.Int64("deleted", deleted))
		}
	}

	return totalDeleted, nil
}
