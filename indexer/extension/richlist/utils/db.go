package utils

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// getCollectedBlock retrieves a block from the database by chain ID and height.
// Uses exponential backoff retry logic for transient database errors.
func GetCollectedBlock(ctx context.Context, db *gorm.DB, chainId string, height int64) (*types.CollectedBlock, error) {
	var block types.CollectedBlock
	var err error

	for attempt := range MAX_ATTEMPTS {
		if err = db.WithContext(ctx).
			Model(&types.CollectedBlock{}).
			Where("chain_id = ?", chainId).
			Where("height = ?", height).First(&block).Error; err != nil {
			ExponentialBackoff(attempt)
			continue
		}
		return &block, nil
	}

	return nil, err
}

// getBlockCollectedTxs retrieves all transactions for a specific block height.
// Returns transactions ordered by sequence in ascending order.
// Uses exponential backoff retry logic for transient database errors.
func GetBlockCollectedTxs(ctx context.Context, db *gorm.DB, height int64) ([]types.CollectedTx, error) {
	var txs []types.CollectedTx
	var err error

	for attempt := range MAX_ATTEMPTS {
		if err := db.WithContext(ctx).
			Model(types.CollectedTx{}).Where("height = ?", height).
			Order("sequence ASC").Find(&txs).Error; err != nil {
			ExponentialBackoff(attempt)
			continue
		}
		return txs, nil
	}

	return nil, err
}

// updateBalanceChanges updates the rich_list table with balance changes.
// It converts addresses to account IDs, updates balances in the database,
// and returns a slice of denoms where any user's balance became negative.
func UpdateBalanceChanges(ctx context.Context, db *gorm.DB, balanceMap map[BalanceChangeKey]sdkmath.Int) ([]string, error) {
	if len(balanceMap) == 0 {
		return nil, nil
	}

	// Step 1: Collect all unique addresses that need account ID conversion
	addressSet := make(map[string]bool)
	for key := range balanceMap {
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
	// Track denoms that have negative balances
	negativeDenoms := make(map[string]bool)

	for key, changeAmount := range balanceMap {
		accountId, ok := accountIdMap[key.Addr]
		if !ok {
			return nil, fmt.Errorf("account ID not found for address: %s", key.Addr)
		}

		// Use raw SQL to update or insert with ON CONFLICT
		// COALESCE is used to handle the case where the record doesn't exist yet (NULL + change = change)
		result := db.WithContext(ctx).Exec(`
			INSERT INTO rich_list (id, denom, amount)
			VALUES (?, ?, ?)
			ON CONFLICT (id, denom)
			DO UPDATE SET amount = (CAST(rich_list.amount AS NUMERIC) + CAST(EXCLUDED.amount AS NUMERIC))::TEXT
		`, accountId, key.Asset, changeAmount.String())

		if result.Error != nil {
			return nil, fmt.Errorf("failed to update balance for account %d, denom %s: %w", accountId, key.Asset, result.Error)
		}

		// Check if the resulting balance is negative
		var updatedAmount string
		err := db.WithContext(ctx).
			Model(&types.CollectedRichList{}).
			Select("amount").
			Where("id = ? AND denom = ?", accountId, key.Asset).
			Scan(&updatedAmount).Error
		if err != nil {
			return nil, fmt.Errorf("failed to check updated balance: %w", err)
		}

		// Parse the amount and check if negative
		amount, ok := sdkmath.NewIntFromString(updatedAmount)
		if !ok {
			return nil, fmt.Errorf("failed to parse amount: %s", updatedAmount)
		}

		if amount.IsNegative() {
			negativeDenoms[key.Asset] = true
		}
	}

	// Step 4: Return list of denoms with negative balances
	result := make([]string, 0, len(negativeDenoms))
	for denom := range negativeDenoms {
		result = append(result, denom)
	}

	return result, nil
}

// getAllAddresses retrieves all unique addresses from the account_dict table.
// Returns a slice of AddressWithID containing both hex-encoded address strings and their account IDs.
func GetAllAddresses(ctx context.Context, db *gorm.DB) ([]AddressWithID, error) {
	var accounts []types.CollectedAccountDict

	if err := db.WithContext(ctx).
		Model(&types.CollectedAccountDict{}).
		Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get addresses from account_dict: %w", err)
	}

	addresses := make([]AddressWithID, len(accounts))
	for i, account := range accounts {
		addresses[i] = AddressWithID{
			Address:   util.BytesToHexWithPrefix(account.Account),
			AccountID: account.Id,
		}
	}

	return addresses, nil
}

// updateRichListStatus updates the rich_list_status table with the current height.
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
	// Use raw SQL to upsert the status record
	// Since there's typically only one row in this table, we can use a simple upsert
	result := db.WithContext(ctx).Exec(`
		INSERT INTO rich_list_status (height)
		VALUES (?)
		ON CONFLICT DO UPDATE SET height = EXCLUDED.height
	`, currentHeight)

	if result.Error != nil {
		return fmt.Errorf("failed to update rich list status: %w", result.Error)
	}

	return nil
}

// updateBalances updates the rich_list table with absolute balance values for a specific denom.
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
		// Use raw SQL to insert or update with ON CONFLICT
		result := db.WithContext(ctx).Exec(`
			INSERT INTO rich_list (id, denom, amount)
			VALUES (?, ?, ?)
			ON CONFLICT (id, denom)
			DO UPDATE SET amount = EXCLUDED.amount
		`, addrWithID.AccountID, denom, balance.String())

		if result.Error != nil {
			return fmt.Errorf("failed to update balance for account %d (address %s), denom %s: %w",
				addrWithID.AccountID, addrWithID.Address, denom, result.Error)
		}
	}

	return nil
}
