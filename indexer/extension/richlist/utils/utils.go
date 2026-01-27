package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	rollytypes "github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/querier"
)

// MAX_ATTEMPTS defines the maximum number of retry attempts for operations with exponential backoff.
const MAX_ATTEMPTS = 10

// ExponentialBackoff sleeps for an exponentially increasing duration based on the attempt number.
// The sleep duration is calculated as: min(2^attempt * 100ms, maxDuration)
// Maximum sleep duration is capped at 5 seconds.
// Returns the context error if the context is cancelled during the sleep.
func ExponentialBackoff(ctx context.Context, attempt int) error {
	const maxDuration = 5 * time.Second
	const baseDelay = 100 * time.Millisecond

	// Calculate exponential delay: 2^attempt * baseDelay
	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay

	// Cap at maximum duration
	delay = min(delay, maxDuration)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// parseHexAmountToSDKInt converts a hex string to sdkmath.Int.
// Expected format: "0x" followed by hex digits.
func ParseHexAmountToSDKInt(data string) (sdkmath.Int, bool) {
	if len(data) < 2 || !strings.HasPrefix(data, "0x") {
		return sdkmath.ZeroInt(), false
	}

	amountBigInt := new(big.Int)
	// Remove "0x" prefix and parse as hex;
	if _, ok := amountBigInt.SetString(data[2:], 16); !ok {
		return sdkmath.ZeroInt(), false
	}
	return sdkmath.NewIntFromBigInt(amountBigInt), true
}

// NewAddressWithID creates an AddressWithID struct from an account address and ID.
// It converts the address to both Bech32 and hex formats.
func NewAddressWithID(address sdk.AccAddress, id int64) AddressWithID {
	return AddressWithID{
		BechAddress: address.String(),
		HexAddress:  util.BytesToHexWithPrefix(address),
		Id:          id,
	}
}

// NewBalanceChangeKey creates a BalanceChangeKey from asset and address.
func NewBalanceChangeKey(denom string, addr sdk.AccAddress) BalanceChangeKey {
	return BalanceChangeKey{
		Denom: denom,
		Addr:  addr.String(),
	}
}

// ApplyBalanceChange adds the amount to the balance map for the denom/address key.
func ApplyBalanceChange(balanceMap map[BalanceChangeKey]sdkmath.Int, denom string, addr sdk.AccAddress, amount sdkmath.Int) {
	key := NewBalanceChangeKey(denom, addr)
	if balance, ok := balanceMap[key]; !ok {
		balanceMap[key] = amount
	} else {
		balanceMap[key] = balance.Add(amount)
	}
}

// ParseCoinsNormalizedDenom parses a coin amount string and normalizes the denomination.
// For EVM chains, it converts the denom to the contract address if available.
func ParseCoinsNormalizedDenom(ctx context.Context, q *querier.Querier, cfg *config.Config, amount string) (sdk.Coins, error) {
	coins, err := sdk.ParseCoinsNormalized(amount)
	if err != nil {
		return nil, err
	}

	for i := range coins {
		denom := strings.ToLower(coins[i].Denom)
		if cfg.GetChainConfig().VmType == rollytypes.EVM {
			contract, err := q.GetEvmContractByDenom(ctx, denom)
			if err != nil {
				continue
			}
			denom = contract
		}
		coins[i].Denom = denom
	}

	return coins, nil
}

// FetchAndUpdateBalances fetches balances for a list of addresses, accumulates them by
// denomination, and updates the rich_list table in the database.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - logger: Logger for progress tracking
//   - db: Database connection for account ID operations and balance updates
//   - cfg: Configuration containing REST API endpoint and other settings
//   - accounts: List of SDK account addresses to fetch balances for
//   - height: The block height to query at
//
// Returns:
//   - error if any step fails
func FetchAndUpdateBalances(
	ctx context.Context,
	q *querier.Querier,
	logger *slog.Logger,
	db *gorm.DB,
	accounts []sdk.AccAddress,
	height int64,
) error {
	accountIdMap, err := getOrCreateAccountIds(db, accounts, true)
	if err != nil {
		return fmt.Errorf("failed to get or create account IDs: %w", err)
	}

	balancesByDenom := make(map[string]map[AddressWithID]sdkmath.Int)
	for idx, address := range accounts {
		if idx%100 == 0 {
			progress := fmt.Sprintf("%d/%d", idx, len(accounts))
			logger.Info("fetching and accumulating balances by denomination", slog.Int64("height", height), slog.String("progress", progress))
		}

		accountID, ok := accountIdMap[address.String()]
		if !ok {
			return fmt.Errorf("account ID not found for address: %s", address)
		}

		// Fetch balances for this account
		balances, err := q.GetAllBalances(ctx, address, height)
		if err != nil {
			return fmt.Errorf("failed to fetch balances for address %s: %w", address, err)
		}

		// Process each balance (denom) for this account
		for _, balance := range balances {
			// Skip zero balances
			if balance.Amount.IsZero() {
				continue
			}

			// Initialize the per-denom map if it doesn't exist
			if balancesByDenom[balance.Denom] == nil {
				balancesByDenom[balance.Denom] = make(map[AddressWithID]sdkmath.Int)
			}

			// Accumulate the balance for this denom
			addrWithID := NewAddressWithID(address, accountID)
			balancesByDenom[balance.Denom][addrWithID] = balance.Amount
		}
	}

	for denom, denomBalances := range balancesByDenom {
		if err := UpdateBalances(ctx, db, denom, denomBalances); err != nil {
			return fmt.Errorf("failed to update balances for denom %s: %w", denom, err)
		}
	}

	return nil
}

// InitializeBalances fetches all accounts, creates account IDs, queries their balances,
// and upserts them to the rich_list table. This is useful for initializing the rich list
// from scratch or syncing absolute balances at a specific block height.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - querier: Querier instance for fetching blockchain data
//   - logger: Logger for progress tracking
//   - db: Database connection for transaction
//   - cfg: Configuration containing REST API endpoint and other settings
//   - height: The block height to query at
//
// Returns:
//   - error if any step fails
func InitializeBalances(ctx context.Context, querier *querier.Querier, logger *slog.Logger, db *gorm.DB, cfg *config.Config, height int64) error {
	// Step 1: Fetch all accounts with pagination
	logger.Info("fetching all accounts with pagination", slog.Int64("height", height))
	accounts, err := querier.FetchAllAccountsWithPagination(ctx, height)
	if err != nil {
		return fmt.Errorf("failed to fetch accounts: %w", err)
	}

	if len(accounts) == 0 {
		logger.Info("no accounts to process", slog.Int64("height", height))
		return nil // No accounts to process
	}

	// Step 2: Fetch balances for each account and update by denomination
	logger.Info("fetching and updating balances", slog.Int64("height", height))
	if err := FetchAndUpdateBalances(ctx, querier, logger, db, accounts, height); err != nil {
		return fmt.Errorf("failed to fetch and accumulate balances: %w", err)
	}

	// Step 3: Update rich list status to track the processed height
	logger.Info("updating rich list status to track the processed height", slog.Int64("height", height))
	if err := UpdateRichListStatus(ctx, db, height); err != nil {
		return fmt.Errorf("failed to update rich list status: %w", err)
	}

	return nil
}

// getOrCreateAccountIds retrieves or creates database IDs for a list of account addresses.
// It converts SDK addresses to strings and delegates to the cache utility.
//
// Parameters:
//   - db: Database connection
//   - accounts: List of account addresses
//   - createNew: If true, creates new IDs for addresses that don't exist in the database
//
// Returns:
//   - idMap: Map of address string to database ID
//   - err: Error if the operation fails
func getOrCreateAccountIds(db *gorm.DB, accounts []sdk.AccAddress, createNew bool) (idMap map[string]int64, err error) {
	var addresses []string
	for _, account := range accounts {
		addresses = append(addresses, account.String())
	}
	return cache.GetOrCreateAccountIds(db, addresses, createNew)
}

// ForEachTxEvents parses tx events and invokes the handler for each tx.
func ForEachTxEvents(txs []rollytypes.CollectedTx, handle func(events sdk.Events)) {
	for _, tx := range txs {
		var txData rollytypes.Tx
		if err := json.Unmarshal(tx.Data, &txData); err != nil {
			continue
		}

		var events sdk.Events
		if err := json.Unmarshal(txData.Events, &events); err != nil {
			continue
		}

		handle(events)
	}
}

// ContainsAddress checks whether the target address exists in the slice.
func ContainsAddress(addresses []sdk.AccAddress, target sdk.AccAddress) bool {
	for _, addr := range addresses {
		if addr.Equals(target) {
			return true
		}
	}
	return false
}
