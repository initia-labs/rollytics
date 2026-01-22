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

// containsAddress checks if an address is in the list of addresses.
func containsAddress(addresses []sdk.AccAddress, target sdk.AccAddress) bool {
	for _, addr := range addresses {
		if addr.Equals(target) {
			return true
		}
	}
	return false
}

// parseCoinsNormalizedDenom parses a coin amount string and normalizes the denomination.
// For EVM chains, it converts the denom to the contract address if available.
func parseCoinsNormalizedDenom(ctx context.Context, querier *querier.Querier, cfg *config.Config, amount string) (sdk.Coins, error) {
	coins, err := sdk.ParseCoinsNormalized(amount)
	if err != nil {
		return nil, err
	}

	for i := range coins {
		denom := strings.ToLower(coins[i].Denom)
		if cfg.GetChainConfig().VmType == types.EVM {
			contract, err := querier.GetEvmContractByDenom(ctx, denom)
			if err != nil {
				continue
			}
			denom = contract
		}
		coins[i].Denom = denom
	}

	return coins, nil
}

// processCosmosMintEvent processes a Cosmos coin mint event and updates the balance map.
// It extracts the minter address and amount from the event, then adds the minted coins to the minter's balance.
func processCosmosMintEvent(ctx context.Context, querier *querier.Querier, logger *slog.Logger, cfg *config.Config, event sdk.Event, balanceMap map[BalanceChangeKey]sdkmath.Int) {
	// Extract attributes from the event
	var minter sdk.AccAddress
	var coins sdk.Coins
	var err error
	for _, attr := range event.Attributes {
		switch attr.Key {
		case "minter":
			if minter, err = util.AccAddressFromString(attr.Value); err != nil {
				logger.Error("failed to parse minter", "minter", attr.Value, "error", err)
			}
		case "amount":
			if coins, err = parseCoinsNormalizedDenom(ctx, querier, cfg, attr.Value); err != nil {
				logger.Error("failed to parse minted coins", "amount", attr.Value, "error", err)
				return
			}
		}
	}

	// Validate required fields are present
	if minter.Empty() {
		logger.Error("invalid minter", "minter", minter)
		return
	}

	// Process each coin in the transfer
	for _, coin := range coins {
		// Update minter's balance (add)
		minterKey := NewBalanceChangeKey(coin.Denom, minter)
		if balance, ok := balanceMap[minterKey]; !ok {
			balanceMap[minterKey] = coin.Amount
		} else {
			balanceMap[minterKey] = balance.Add(coin.Amount)
		}
	}
}

// processCosmosBurnEvent processes a Cosmos coin burn event and updates the balance map.
// It extracts the burner address and amount from the event, then subtracts the burned coins from the burner's balance.
func processCosmosBurnEvent(ctx context.Context, querier *querier.Querier, logger *slog.Logger, cfg *config.Config, event sdk.Event, balanceMap map[BalanceChangeKey]sdkmath.Int) {
	// Extract attributes from the event
	var burner sdk.AccAddress
	var coins sdk.Coins
	var err error
	for _, attr := range event.Attributes {
		switch attr.Key {
		case "burner":
			if burner, err = util.AccAddressFromString(attr.Value); err != nil {
				logger.Error("failed to parse burner", "burner", attr.Value, "error", err)
			}
		case "amount":
			if coins, err = parseCoinsNormalizedDenom(ctx, querier, cfg, attr.Value); err != nil {
				logger.Error("failed to parse burned coins", "amount", attr.Value, "error", err)
				return
			}
		}
	}

	// Validate required fields are present
	if burner.Empty() {
		logger.Error("invalid burner", "burner", burner)
		return
	}

	// Process each coin in the transfer
	for _, coin := range coins {
		// Update burner's balance (subtract)
		burnerKey := NewBalanceChangeKey(coin.Denom, burner)
		if balance, ok := balanceMap[burnerKey]; !ok {
			balanceMap[burnerKey] = coin.Amount.Neg()
		} else {
			balanceMap[burnerKey] = balance.Sub(coin.Amount)
		}
	}
}

// processCosmosTransferEvent processes a Cosmos transfer event and updates the balance map.
// It extracts transfer information from the event attributes and updates balances for both sender and receiver.
// Module accounts are excluded from balance tracking to avoid tracking treasury and system accounts.
func processCosmosTransferEvent(ctx context.Context, querier *querier.Querier, logger *slog.Logger, cfg *config.Config, event sdk.Event, balanceMap map[BalanceChangeKey]sdkmath.Int, moduleAccounts []sdk.AccAddress) {
	// Extract attributes from the event
	var recipient, sender sdk.AccAddress
	var coins sdk.Coins
	var err error
	for _, attr := range event.Attributes {
		switch attr.Key {
		case "recipient":
			if recipient, err = util.AccAddressFromString(attr.Value); err != nil {
				logger.Error("failed to parse recipient", "recipient", attr.Value, "error", err)
			}
		case "sender":
			if sender, err = util.AccAddressFromString(attr.Value); err != nil {
				logger.Error("failed to parse sender", "sender", attr.Value, "error", err)
			}
		case "amount":
			if coins, err = parseCoinsNormalizedDenom(ctx, querier, cfg, attr.Value); err != nil {
				logger.Error("failed to parse transferred coins", "amount", attr.Value, "error", err)
				return
			}
		}
	}

	// Validate required fields are present
	if recipient.Empty() || sender.Empty() {
		logger.Error("invalid either recipient or sender", "recipient", recipient, "sender", sender)
		return
	}

	// Process each coin in the transfer
	for _, coin := range coins {
		if !containsAddress(moduleAccounts, sender) {
			// Update sender's balance (subtract)
			senderKey := NewBalanceChangeKey(coin.Denom, sender)
			if balance, ok := balanceMap[senderKey]; !ok {
				balanceMap[senderKey] = coin.Amount.Neg()
			} else {
				balanceMap[senderKey] = balance.Sub(coin.Amount)
			}
		}

		if !containsAddress(moduleAccounts, recipient) {
			// Update recipient's balance (add)
			recipientKey := NewBalanceChangeKey(coin.Denom, recipient)
			if balance, ok := balanceMap[recipientKey]; !ok {
				balanceMap[recipientKey] = coin.Amount
			} else {
				balanceMap[recipientKey] = balance.Add(coin.Amount)
			}
		}
	}
}

// processMoveTransferEvents processes Move VM transfer events (deposit/withdraw) and updates the balance map.
// It handles fungible asset transfers in the Move primary store by matching deposit/withdraw events with their owner events.
// Module accounts are excluded from balance tracking.
func processMoveTransferEvents(ctx context.Context, querier *querier.Querier, logger *slog.Logger, events sdk.Events, balanceMap map[BalanceChangeKey]sdkmath.Int, moduleAccounts []sdk.AccAddress) {
	for idx, event := range events {
		if idx == len(events)-1 || event.Type != "move" || len(event.Attributes) < 2 || event.Attributes[0].Key != "type_tag" || len(events[idx+1].Attributes) < 2 || events[idx+1].Attributes[0].Key != "type_tag" {
			continue
		}

		// Support only Fungible Asset in primary store (always following with an owner event)
		// - 0x1::fungible_asset::DepositEvent => 0x1::fungible_asset::DepositOwnerEvent
		// - 0x1::fungible_asset::WithdrawEvent => 0x1::fungible_asset::WithdrawOwnerEvent
		if event.Attributes[0].Value == types.MoveDepositEventTypeTag && events[idx+1].Attributes[0].Value == types.MoveDepositOwnerEventTypeTag {
			var depositEvent MoveDepositEvent
			err := json.Unmarshal([]byte(event.Attributes[1].Value), &depositEvent)
			if err != nil {
				logger.Error("failed to unmarshal deposit event", "error", err)
				continue
			}

			var depositOwnerEvent MoveDepositOwnerEvent
			err = json.Unmarshal([]byte(events[idx+1].Attributes[1].Value), &depositOwnerEvent)
			if err != nil {
				logger.Error("failed to unmarshal deposit owner event", "error", err)
				continue
			}

			recipient, err := util.AccAddressFromString(depositOwnerEvent.Owner)
			if err != nil {
				logger.Error("failed to parse recipient", "recipient", depositOwnerEvent.Owner, "error", err)
				continue
			}
			denom, err := querier.GetMoveDenomByMetadataAddr(ctx, depositEvent.MetadataAddr)
			if err != nil {
				logger.Error("failed to get move denom", "metadataAddr", depositEvent.MetadataAddr, "error", err)
				continue
			}
			amount, ok := sdkmath.NewIntFromString(depositEvent.Amount)
			if !ok {
				logger.Error("failed to parse coin", "coin", depositEvent.Amount, "error", err)
				continue
			}

			if !containsAddress(moduleAccounts, recipient) {
				// Update recipient's balance (add)
				recipientKey := NewBalanceChangeKey(denom, recipient)
				if balance, ok := balanceMap[recipientKey]; !ok {
					balanceMap[recipientKey] = amount
				} else {
					balanceMap[recipientKey] = balance.Add(amount)
				}
			}
		}

		if event.Attributes[0].Value == types.MoveWithdrawEventTypeTag && events[idx+1].Attributes[0].Value == types.MoveWithdrawOwnerEventTypeTag {
			var withdrawEvent MoveWithdrawEvent
			err := json.Unmarshal([]byte(event.Attributes[1].Value), &withdrawEvent)
			if err != nil {
				logger.Error("failed to unmarshal withdraw event", "error", err)
				continue
			}

			var withdrawOwnerEvent MoveWithdrawOwnerEvent
			err = json.Unmarshal([]byte(events[idx+1].Attributes[1].Value), &withdrawOwnerEvent)
			if err != nil {
				logger.Error("failed to unmarshal withdraw owner event", "error", err)
				continue
			}

			sender, err := util.AccAddressFromString(withdrawOwnerEvent.Owner)
			if err != nil {
				logger.Error("failed to parse sender", "sender", withdrawOwnerEvent.Owner, "error", err)
				continue
			}
			denom, err := querier.GetMoveDenomByMetadataAddr(ctx, withdrawEvent.MetadataAddr)
			if err != nil {
				logger.Error("failed to get move denom", "metadataAddr", withdrawEvent.MetadataAddr, "error", err)
				continue
			}
			amount, ok := sdkmath.NewIntFromString(withdrawEvent.Amount)
			if !ok {
				logger.Error("failed to parse coin", "coin", withdrawEvent.Amount, "error", err)
				continue
			}

			if !containsAddress(moduleAccounts, sender) {
				// Update sender's balance (subtract)
				senderKey := NewBalanceChangeKey(denom, sender)
				if balance, ok := balanceMap[senderKey]; !ok {
					balanceMap[senderKey] = amount.Neg()
				} else {
					balanceMap[senderKey] = balance.Sub(amount)
				}
			}
		}
	}
}

// processEvmTransferEvents processes EVM events and updates the balance map.
// It extracts the EVM log from the event's "log" attribute, parses the JSON-encoded log,
// validates it's an ERC20 Transfer event (by checking the transfer topic), and updates balances
// for both sender (subtract) and receiver (add). The empty address (0x0) is skipped for mint/burn operations.
func processEvmTransferEvents(logger *slog.Logger, events sdk.Events, balanceMap map[BalanceChangeKey]sdkmath.Int) {
	// Extract the log attribute from the event
	for _, event := range events {
		for _, attr := range event.Attributes {
			if attr.Key == "log" {
				// Parse the JSON log
				var evmLog EvmEventLog
				if err := json.Unmarshal([]byte(attr.Value), &evmLog); err != nil {
					logger.Error("failed to unmarshal evm log", "error", err)
					continue
				}

				// Validate it's a Transfer event with the correct number of topics
				if len(evmLog.Topics) != 3 || evmLog.Topics[0] != types.EvmTransferTopic {
					continue
				}

				denom := strings.ToLower(evmLog.Address)
				fromAddr := evmLog.Topics[1]
				toAddr := evmLog.Topics[2]

				// Parse amount from hex string in evmLog.Data
				amount, ok := ParseHexAmountToSDKInt(evmLog.Data)
				if !ok {
					logger.Error("failed to parse amount from evm log data", "data", evmLog.Data)
					continue
				}

				// Update sender's balance (subtract)
				if fromAccAddr, err := util.AccAddressFromString(fromAddr); fromAddr != types.EvmEmptyAddress && err == nil {
					fromKey := NewBalanceChangeKey(denom, fromAccAddr)
					if balance, exists := balanceMap[fromKey]; !exists {
						balanceMap[fromKey] = amount.Neg()
					} else {
						balanceMap[fromKey] = balance.Sub(amount)
					}
				}

				// Update receiver's balance (add)
				if toAccAddr, err := util.AccAddressFromString(toAddr); toAddr != types.EvmEmptyAddress && err == nil {
					toKey := NewBalanceChangeKey(denom, toAccAddr)
					if balance, exists := balanceMap[toKey]; !exists {
						balanceMap[toKey] = amount
					} else {
						balanceMap[toKey] = balance.Add(amount)
					}
				}
			}
		}
	}
}

// ProcessBalanceChanges processes blockchain transactions and calculates balance changes for each address.
// It routes events to the appropriate processor based on the chain's VM type (MoveVM, EVM, or Cosmos).
// Returns a map of BalanceChangeKey to balance change amounts (positive for increases, negative for decreases).
func ProcessBalanceChanges(ctx context.Context, querier *querier.Querier, logger *slog.Logger, cfg *config.Config, txs []types.CollectedTx, moduleAccounts []sdk.AccAddress) map[BalanceChangeKey]sdkmath.Int {
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

		switch cfg.GetChainConfig().VmType {
		case types.MoveVM:
			processMoveTransferEvents(ctx, querier, logger, events, balanceMap, moduleAccounts)
		case types.EVM:
			processEvmTransferEvents(logger, events, balanceMap)
		default:
			for _, event := range events {
				switch event.Type {
				case banktypes.EventTypeCoinMint:
					processCosmosMintEvent(ctx, querier, logger, cfg, event, balanceMap)
				case banktypes.EventTypeCoinBurn:
					processCosmosBurnEvent(ctx, querier, logger, cfg, event, balanceMap)
				case banktypes.EventTypeTransfer:
					processCosmosTransferEvent(ctx, querier, logger, cfg, event, balanceMap, moduleAccounts)
				}
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
