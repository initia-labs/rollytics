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
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
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

// containsAddress checks if an address is in the list of addresses.
func containsAddress(addresses []sdk.AccAddress, target sdk.AccAddress) bool {
	for _, addr := range addresses {
		if addr.Equals(target) {
			return true
		}
	}
	return false
}

// processCosmosTransferEvent processes a Cosmos transfer event and updates the balance map.
// It extracts transfer information from the event attributes and updates balances for both sender and receiver.
// Returns true if the event was successfully processed, false otherwise.
func processCosmosTransferEvent(logger *slog.Logger, cfg *config.Config, event sdk.Event, balanceMap map[BalanceChangeKey]sdkmath.Int, moduleAccounts []sdk.AccAddress) bool {
	// Extract attributes from the event
	var recipient, sender sdk.AccAddress
	var amount string
	for _, attr := range event.Attributes {
		switch attr.Key {
		case "recipient":
			if accAddress, err := util.AccAddressFromString(attr.Value); err == nil {
				recipient = accAddress
			}
		case "sender":
			if accAddress, err := util.AccAddressFromString(attr.Value); err == nil {
				sender = accAddress
			}
		case "amount":
			amount = attr.Value
		}
	}

	// Validate required fields are present
	if recipient.Empty() || sender.Empty() || amount == "" {
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
		denom := strings.ToLower(strings.ToLower(coin.Denom))
		if cfg.GetChainConfig().VmType == types.EVM {
			contract, err := util.GetEvmContractByDenom(context.Background(), denom)
			if err != nil {
				continue
			}
			denom = contract
		}

		if !containsAddress(moduleAccounts, sender) {
			// Update sender's balance (subtract)
			senderKey := NewBalanceChangeKey(denom, sender)
			if balance, ok := balanceMap[senderKey]; !ok {
				balanceMap[senderKey] = sdkmath.ZeroInt().Sub(coin.Amount)
			} else {
				balanceMap[senderKey] = balance.Sub(coin.Amount)
			}
		}

		if !containsAddress(moduleAccounts, recipient) {
			// Update recipient's balance (add)
			recipientKey := NewBalanceChangeKey(denom, recipient)
			if balance, ok := balanceMap[recipientKey]; !ok {
				balanceMap[recipientKey] = coin.Amount
			} else {
				balanceMap[recipientKey] = balance.Add(coin.Amount)
			}
		}
	}

	return true
}

// processEvmTransferEvent processes an "evm" type event and updates the balance map.
// It extracts the EVM log from the event's "log" attribute, parses the JSON-encoded log,
// validates it's an ERC20 Transfer event, and updates balances for both sender (subtract) and receiver (add).
// Returns true if the event was successfully processed, false otherwise.
func processEvmTransferEvent(logger *slog.Logger, event sdk.Event, balanceMap map[BalanceChangeKey]sdkmath.Int) bool {
	// Extract the log attribute from the event
	for _, attr := range event.Attributes {
		if attr.Key == "log" {
			// Parse the JSON log
			var evmLog EvmEventLog
			if err := json.Unmarshal([]byte(attr.Value), &evmLog); err != nil {
				logger.Error("failed to unmarshal evm log", "error", err)
				return false
			}

			// Validate it's a Transfer event with the correct number of topics
			if len(evmLog.Topics) != 3 || evmLog.Topics[0] != EVM_TRANSFER_TOPIC {
				return false
			}

			denom := strings.ToLower(evmLog.Address)
			fromAddr := evmLog.Topics[1]
			toAddr := evmLog.Topics[2]

			// Parse amount from hex string in evmLog.Data
			amount, ok := ParseHexAmountToSDKInt(evmLog.Data)
			if !ok {
				logger.Error("failed to parse amount from evm log data", "data", evmLog.Data)
				return false
			}

			// Update sender's balance (subtract)
			if fromAccAddr, err := util.AccAddressFromString(fromAddr); fromAddr != EMPTY_ADDRESS && err == nil {
				fromKey := NewBalanceChangeKey(denom, fromAccAddr)
				if balance, exists := balanceMap[fromKey]; !exists {
					balanceMap[fromKey] = sdkmath.ZeroInt().Sub(amount)
				} else {
					balanceMap[fromKey] = balance.Sub(amount)
				}
			}

			// Update receiver's balance (add)
			if toAccAddr, err := util.AccAddressFromString(toAddr); toAddr != EMPTY_ADDRESS && err == nil {
				toKey := NewBalanceChangeKey(denom, toAccAddr)
				if balance, exists := balanceMap[toKey]; !exists {
					balanceMap[toKey] = amount
				} else {
					balanceMap[toKey] = balance.Add(amount)
				}
			}
		}
	}

	return true
}

// ProcessCosmosBalanceChanges processes Cosmos transactions and calculates balance changes
// for each address. Returns a map of BalanceChangeKey to balance change amounts.
func ProcessCosmosBalanceChanges(logger *slog.Logger, cfg *config.Config, txs []types.CollectedTx, moduleAccounts []sdk.AccAddress) map[BalanceChangeKey]sdkmath.Int {
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
			if event.Type == banktypes.EventTypeTransfer && cfg.GetChainConfig().VmType != types.EVM {
				processCosmosTransferEvent(logger, cfg, event, balanceMap, moduleAccounts)
			} else if event.Type == "evm" {
				processEvmTransferEvent(logger, event, balanceMap)
			}
		}
	}

	return balanceMap
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
	logger *slog.Logger,
	db *gorm.DB,
	cfg *config.Config,
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
		balances, err := fetchAccountBalancesWithPagination(ctx, cfg, address, height)
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

// initializeBalances fetches all accounts, creates account IDs, queries their balances,
// and upserts them to the rich_list table. This is useful for initializing the rich list
// from scratch or syncing absolute balances.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - logger: Logger for progress tracking
//   - db: Database connection for transaction
//   - cfg: Configuration containing REST API endpoint and other settings
//   - height: The block height to query at
//
// Returns:
//   - error if any step fails
func InitializeBalances(ctx context.Context, logger *slog.Logger, db *gorm.DB, cfg *config.Config, height int64) error {
	// Step 1: Fetch all accounts with pagination
	logger.Info("fetching all accounts with pagination", slog.Int64("height", height))
	accounts, err := fetchAllAccountsWithPagination(ctx, cfg, height)
	if err != nil {
		return fmt.Errorf("failed to fetch accounts: %w", err)
	}

	if len(accounts) == 0 {
		logger.Info("no accounts to process", slog.Int64("height", height))
		return nil // No accounts to process
	}

	// Step 2: Fetch balances for each account and update by denomination
	logger.Info("fetching and updating balances", slog.Int64("height", height))
	if err := FetchAndUpdateBalances(ctx, logger, db, cfg, accounts, height); err != nil {
		return fmt.Errorf("failed to fetch and accumulate balances: %w", err)
	}

	// Step 3: Update rich list status to track the processed height
	logger.Info("updating rich list status to track the processed height", slog.Int64("height", height))
	if err := UpdateRichListStatus(ctx, db, height); err != nil {
		return fmt.Errorf("failed to update rich list status: %w", err)
	}

	return nil
}

func getOrCreateAccountIds(db *gorm.DB, accounts []sdk.AccAddress, createNew bool) (idMap map[string]int64, err error) {
	var addresses []string
	for _, account := range accounts {
		addresses = append(addresses, account.String())
	}
	return util.GetOrCreateAccountIds(db, addresses, createNew)
}
