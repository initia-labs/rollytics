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

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

const MAX_ATTEMPTS = 10

// ExponentialBackoff sleeps for an exponentially increasing duration based on the attempt number.
// The sleep duration is calculated as: min(2^attempt * 100ms, maxDuration)
// Maximum sleep duration is capped at 5 seconds.
func ExponentialBackoff(attempt int) {
	const maxDuration = 5 * time.Second
	const baseDelay = 100 * time.Millisecond

	// Calculate exponential delay: 2^attempt * baseDelay
	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay

	// Cap at maximum duration
	delay = min(delay, maxDuration)

	time.Sleep(delay)
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

// NewBalanceChangeKey creates a BalanceChangeKey from asset and address.
func NewBalanceChangeKey(asset, addr string) BalanceChangeKey {
	return BalanceChangeKey{
		Asset: asset,
		Addr:  addr,
	}
}

// processCosmosTransferEvent processes a Cosmos transfer event and updates the balance map.
// It extracts transfer information from the event attributes and updates balances for both sender and receiver.
// Returns true if the event was successfully processed, false otherwise.
func processCosmosTransferEvent(logger *slog.Logger, event sdk.Event, balanceMap map[BalanceChangeKey]sdkmath.Int) bool {
	// Extract attributes from the event
	var recipient, sender, amount string
	for _, attr := range event.Attributes {
		switch attr.Key {
		case "recipient":
			recipient = attr.Value
		case "sender":
			sender = attr.Value
		case "amount":
			amount = attr.Value
		}
	}

	// Validate required fields are present
	if recipient == "" || sender == "" || amount == "" {
		return false
	}

	// Parse the amount string which can contain multiple coins (e.g., "100uinit,200utoken")
	coins, err := sdk.ParseCoinsNormalized(amount)
	if err != nil {
		logger.Error("failed to parse coins", "amount", amount, "error", err)
		return false
	}

	// Process each coin in the transfer
	for _, coin := range coins {
		// Update sender's balance (subtract)
		senderKey := NewBalanceChangeKey(coin.Denom, sender)
		if balance, ok := balanceMap[senderKey]; !ok {
			balanceMap[senderKey] = sdkmath.ZeroInt().Sub(coin.Amount)
		} else {
			balanceMap[senderKey] = balance.Sub(coin.Amount)
		}

		// Update recipient's balance (add)
		recipientKey := NewBalanceChangeKey(coin.Denom, recipient)
		if balance, ok := balanceMap[recipientKey]; !ok {
			balanceMap[recipientKey] = coin.Amount
		} else {
			balanceMap[recipientKey] = balance.Add(coin.Amount)
		}
	}

	return true
}

// ProcessCosmosBalanceChanges processes Cosmos transactions and calculates balance changes
// for each address. Returns a map of BalanceChangeKey to balance change amounts.
func ProcessCosmosBalanceChanges(logger *slog.Logger, txs []types.CollectedTx) map[BalanceChangeKey]sdkmath.Int {
	balanceMap := make(map[BalanceChangeKey]sdkmath.Int)

	// Process each transaction
	for _, tx := range txs {
		// Parse tx data to get timestamp and events
		var txData types.Tx
		if err := json.Unmarshal(tx.Data, &txData); err != nil {
			continue
		}

		var events sdk.Events
		if err := json.Unmarshal(txData.Events, &events); err != nil {
			continue
		}

		for _, event := range events {
			if event.Type == COSMOS_TRANSFER_EVENT {
				processCosmosTransferEvent(logger, event, balanceMap)
			}
		}
	}

	return balanceMap
}

// initializeBalances fetches all accounts, creates account IDs, queries their balances,
// and upserts them to the rich_list table. This is useful for initializing the rich list
// from scratch or syncing absolute balances.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - db: Database connection for transaction
//   - restURL: The Cosmos SDK REST API endpoint URL
//   - height: The block height to query at
//
// Returns:
//   - error if any step fails
func InitializeBalances(ctx context.Context, logger *slog.Logger, db *gorm.DB, restURL string, height int64) error {
	// Step 1: Fetch all accounts with pagination
	logger.Info("fetching all accounts with pagination", slog.Int64("height", height))
	addresses, err := fetchAllAccountsWithPagination(ctx, restURL, height)
	if err != nil {
		return fmt.Errorf("failed to fetch accounts: %w", err)
	}

	if len(addresses) == 0 {
		return nil // No accounts to process
	}

	// Step 2: Get or create account IDs
	logger.Info("getting or creating account IDs", slog.Int64("height", height), slog.Int("num_accounts", len(addresses)))
	accountIDMap, err := util.GetOrCreateAccountIds(db, addresses, true)
	if err != nil {
		return fmt.Errorf("failed to get or create account IDs: %w", err)
	}

	// Step 3: Fetch balances for each account and accumulate by denomination
	// Map structure: denom -> (AddressWithID -> amount)
	balancesByDenom := make(map[string]map[AddressWithID]sdkmath.Int)

	for idx, address := range addresses {
		if idx%100 == 0 {
			progress := fmt.Sprintf("%d/%d", idx, len(addresses))
			logger.Info("fetching balances for each account and accumulating by denomination", slog.Int64("height", height), slog.String("progress", progress))
		}

		accountID, ok := accountIDMap[address]
		if !ok {
			return fmt.Errorf("account ID not found for address: %s", address)
		}

		// Fetch balances for this account
		balances, err := fetchAccountBalancesWithPagination(ctx, restURL, address, height)
		if err != nil {
			return fmt.Errorf("failed to fetch balances for address %s: %w", address, err)
		}

		// Process each balance (denom) for this account
		for _, balance := range balances {
			amount, ok := sdkmath.NewIntFromString(balance.Amount)
			if !ok {
				return fmt.Errorf("failed to parse amount for address %s, denom %s: %s", address, balance.Denom, balance.Amount)
			}

			// Skip zero balances
			if amount.IsZero() {
				continue
			}

			addrWithID := AddressWithID{
				Address:   address,
				AccountID: accountID,
			}

			// Initialize the per-denom map if it doesn't exist
			if balancesByDenom[balance.Denom] == nil {
				balancesByDenom[balance.Denom] = make(map[AddressWithID]sdkmath.Int)
			}

			// Accumulate the balance for this denom
			balancesByDenom[balance.Denom][addrWithID] = amount
		}
	}

	// Step 4: Batch update balances by denomination
	logger.Info("batch updating balances by denomination", slog.Int64("height", height), slog.Int("num_denoms", len(balancesByDenom)))
	for denom, denomBalances := range balancesByDenom {
		if err := UpdateBalances(ctx, db, denom, denomBalances); err != nil {
			return fmt.Errorf("failed to update balances for denom %s: %w", denom, err)
		}
	}

	// Step 5: Update rich list status to track the processed height
	logger.Info("updating rich list status to track the processed height", slog.Int64("height", height))
	if err := UpdateRichListStatus(ctx, db, height); err != nil {
		return fmt.Errorf("failed to update rich list status: %w", err)
	}

	return nil
}
