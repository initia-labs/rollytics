package utils

import (
	"context"
	"errors"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

const RICH_LIST_BLOCK_DELAY = 3

// GetLatestCollectedBlock retrieves the latest block height from the database for a given chain ID.
func GetLatestCollectedBlock(ctx context.Context, db *gorm.DB, chainId string) (int64, error) {
	var latestHeight int64
	err := db.WithContext(ctx).
		Model(&types.CollectedBlock{}).
		Where("chain_id = ?", chainId).
		Select("MAX(height)").
		Scan(&latestHeight).Error
	if err != nil {
		return 0, err
	}
	return latestHeight, nil
}

// GetCollectedBlock retrieves a block from the database by chain ID and height.
// Uses exponential backoff retry logic for transient database errors.
// GetCollectedBlock retrieves a block from the database by chain ID and height.
// Uses exponential backoff retry logic for transient database errors.
func GetCollectedBlock(ctx context.Context, db *gorm.DB, chainId string, height int64) (*types.CollectedBlock, error) {
	var block types.CollectedBlock
	for attempt := 0; ; attempt++ {
		err := db.WithContext(ctx).
			Model(&types.CollectedBlock{}).
			Where("chain_id = ?", chainId).
			Where("height = ?", height).
			Omit("timestamp").
			First(&block).Error
		if err == nil {
			// Check if the block is safe to process (latest height - height > BlockDelay)
			latestHeight, err := GetLatestCollectedBlock(ctx, db, chainId)
			if err != nil {
				// If we can't get the latest height, we should probably retry or error out.
				// For now, let's treat it as a transient error and retry.
				ExponentialBackoff(attempt)
				continue
			}

			if latestHeight-height > RICH_LIST_BLOCK_DELAY {
				return &block, nil
			}

			// Block is too recent, wait and retry
			ExponentialBackoff(attempt)
			continue
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Record not found, retry with exponential backoff
			ExponentialBackoff(attempt)
			continue
		} else {
			// For other errors, return immediately
			return nil, err
		}
	}
}

// GetBlockCollectedTxs retrieves all transactions for a specific block height.
// Returns transactions ordered by sequence in ascending order.
func GetBlockCollectedTxs(ctx context.Context, db *gorm.DB, height int64) ([]types.CollectedTx, error) {
	var txs []types.CollectedTx

	if err := db.WithContext(ctx).
		Model(types.CollectedTx{}).Where("height = ?", height).
		Order("sequence ASC").Find(&txs).Error; err != nil {
		return nil, err
	}

	return txs, nil
}

// UpdateBalanceChanges updates the rich_list table with balance changes.
// It converts addresses to account IDs, updates balances in the database,
// and returns a slice of denoms where any user's balance became negative.
func UpdateBalanceChanges(ctx context.Context, db *gorm.DB, balanceMap map[BalanceChangeKey]sdkmath.Int) ([]string, error) {
	if len(balanceMap) == 0 {
		return nil, nil
	}

	// Step 1: Collect all unique addresses that need account ID conversion
	addressSet := make(map[string]bool)
	for key := range balanceMap {
		if len(key.Addr) > 44 {
			continue
		}
		addressSet[key.Addr] = true
	}

	addresses := make([]string, 0, len(addressSet))
	for addr := range addressSet {
		addresses = append(addresses, addr)
	}

	// Step 2: Convert addresses to account IDs
	accountIdMap, err := util.GetOrCreateAccountIds(db, addresses, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create account IDs: %w", err)
	}

	// Step 3: Update balances in the database using raw SQL for atomic updates
	// Track denoms that have negative balances. Negative balances can occur due to
	// transaction ordering issues or missed events, and require correction via on-chain queries.
	negativeDenoms := make(map[string]bool)

	for key, changeAmount := range balanceMap {
		accountId, ok := accountIdMap[key.Addr]
		if !ok {
			return nil, fmt.Errorf("account ID not found for address: %s", key.Addr)
		}

		// Use raw SQL to update or insert with ON CONFLICT and RETURNING to get the updated amount in one query
		var updatedAmount string
		err := db.WithContext(ctx).Raw(`
			INSERT INTO rich_list (id, denom, amount)
			VALUES (?, ?, ?::numeric)
			ON CONFLICT (id, denom)
			DO UPDATE SET amount = rich_list.amount + EXCLUDED.amount
			RETURNING amount::text
		`, accountId, key.Denom, changeAmount.String()).Scan(&updatedAmount).Error
		if err != nil {
			return nil, fmt.Errorf("failed to update balance for account %d, denom %s: %w", accountId, key.Denom, err)
		}

		// Parse the amount and check if negative
		amount, ok := sdkmath.NewIntFromString(updatedAmount)
		if !ok {
			return nil, fmt.Errorf("failed to parse amount: %s", updatedAmount)
		}

		if amount.IsNegative() {
			negativeDenoms[key.Denom] = true
		}
	}

	// Step 4: Return list of denoms with negative balances
	result := make([]string, 0, len(negativeDenoms))
	for denom := range negativeDenoms {
		result = append(result, denom)
	}

	return result, nil
}

// GetAllAddresses retrieves all unique addresses from the account_dict table.
// Returns a slice of AddressWithID containing both hex-encoded address strings and their account IDs.
func GetAllAddresses(ctx context.Context, db *gorm.DB, vmType types.VMType) ([]AddressWithID, error) {
	var accounts []types.CollectedAccountDict

	if err := db.WithContext(ctx).
		Model(&types.CollectedAccountDict{}).
		Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get addresses from account_dict: %w", err)
	}

	addresses := make([]AddressWithID, 0)
	for _, account := range accounts {
		if vmType == types.EVM && len(account.Account) > 20 {
			continue
		}
		addresses = append(addresses, NewAddressWithID(account.Account, account.Id))
	}

	return addresses, nil
}

// UpdateRichListStatus updates the rich_list_status table with the current height.
// This should be called before incrementing the height to track progress of the rich list indexer.
//
// Parameters:
//   - ctx: Context for database operations
//   - db: Database connection
//   - currentHeight: The current block height being processed
//
// Returns:
//   - error if the update fails
func UpdateRichListStatus(ctx context.Context, db *gorm.DB, currentHeight int64) error {
	var status types.CollectedRichListStatus
	result := db.WithContext(ctx).First(&status)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// No record exists, create it
			result = db.WithContext(ctx).Create(&types.CollectedRichListStatus{Height: currentHeight})
		} else {
			// Some other error occurred
			return result.Error
		}
	} else {
		// Record exists, update it
		result = db.WithContext(ctx).Model(&status).Where("1 = 1").Updates(types.CollectedRichListStatus{Height: currentHeight})
	}
	if result.Error != nil {
		return fmt.Errorf("failed to update rich list status: %w", result.Error)
	}

	return nil
}

// UpdateBalances updates the rich_list table with absolute balance values for a specific denom.
// It takes a map of AddressWithID to their absolute balances (not deltas) and updates the database.
//
// Parameters:
//   - ctx: Context for database operations
//   - db: Database connection
//   - denom: The ERC20 token contract address or asset denom
//   - addressBalances: Map of AddressWithID to absolute balance amount
//
// Returns:
//   - error if the update fails
//
// The function uses the account IDs from AddressWithID and uses UPSERT to insert or update balances.
func UpdateBalances(ctx context.Context, db *gorm.DB, denom string, addressBalances map[AddressWithID]sdkmath.Int) error {
	if len(addressBalances) == 0 {
		return nil
	}

	// Update balances in the database using raw SQL for atomic updates
	for addrWithID, balance := range addressBalances {
		if len(addrWithID.BechAddress) > 44 {
			continue
		}
		// Use raw SQL to insert or update with ON CONFLICT
		result := db.WithContext(ctx).Exec(`
			INSERT INTO rich_list (id, denom, amount)
			VALUES (?, ?, ?)
			ON CONFLICT (id, denom)
			DO UPDATE SET amount = EXCLUDED.amount
		`, addrWithID.Id, denom, balance.String())

		if result.Error != nil {
			return fmt.Errorf("failed to update balance for account %d (address %s), denom %s: %w",
				addrWithID.Id, addrWithID.BechAddress, denom, result.Error)
		}
	}

	return nil
}

func QueryBalance(ctx context.Context, db *gorm.DB, denom string, address string) (types.CollectedRichList, error) {
	var accountId int64
	idMap, err := util.GetOrCreateAccountIds(db, []string{address}, false)
	if err != nil {
		return types.CollectedRichList{}, fmt.Errorf("failed to get account ID: %w", err)
	}
	accountId, ok := idMap[address]
	if !ok {
		return types.CollectedRichList{}, fmt.Errorf("account ID not found for address: %s", address)
	}

	var balance types.CollectedRichList
	if err := db.WithContext(ctx).Model(&types.CollectedRichList{}).Where("denom = ? AND id = ?", denom, accountId).First(&balance).Error; err != nil {
		return types.CollectedRichList{}, fmt.Errorf("failed to query balance: %w", err)
	}
	return balance, nil
}
