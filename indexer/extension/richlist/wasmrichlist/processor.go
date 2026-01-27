package wasmrichlist

import (
	"context"
	"log/slog"
	"strings"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	richlisttypes "github.com/initia-labs/rollytics/indexer/extension/richlist/types"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	rollytypes "github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/querier"
)

var _ richlisttypes.RichListProcessor = (*RichList)(nil)

type RichList struct {
	cfg *config.Config
}

func New(cfg *config.Config) *RichList {
	return &RichList{
		cfg: cfg,
	}
}

func (r *RichList) ProcessBalanceChanges(
	ctx context.Context,
	q *querier.Querier,
	logger *slog.Logger,
	txs []rollytypes.CollectedTx,
	moduleAccounts []sdk.AccAddress,
) map[richlistutils.BalanceChangeKey]sdkmath.Int {
	balanceMap := make(map[richlistutils.BalanceChangeKey]sdkmath.Int)

	richlistutils.ForEachTxEvents(txs, func(events sdk.Events) {
		for _, event := range events {
			switch event.Type {
			case banktypes.EventTypeCoinMint:
				processCosmosMintEvent(ctx, q, logger, r.cfg, event, balanceMap)
			case banktypes.EventTypeCoinBurn:
				processCosmosBurnEvent(ctx, q, logger, r.cfg, event, balanceMap)
			case banktypes.EventTypeTransfer:
				processCosmosTransferEvent(ctx, q, logger, r.cfg, event, balanceMap, moduleAccounts)
			}
		}
	})

	return balanceMap
}

func (r *RichList) AfterProcess(_ context.Context, _ *gorm.DB, _ int64, _ []string, _ *querier.Querier) error {
	return nil
}

// parseCoinsNormalizedDenom parses a coin amount string and normalizes the denomination.
// For EVM chains, it converts the denom to the contract address if available.
func parseCoinsNormalizedDenom(ctx context.Context, q *querier.Querier, cfg *config.Config, amount string) (sdk.Coins, error) {
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

func processCosmosMintEvent(ctx context.Context, q *querier.Querier, logger *slog.Logger, cfg *config.Config, event sdk.Event, balanceMap map[richlistutils.BalanceChangeKey]sdkmath.Int) {
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
			if coins, err = parseCoinsNormalizedDenom(ctx, q, cfg, attr.Value); err != nil {
				logger.Error("failed to parse minted coins", "amount", attr.Value, "error", err)
				return
			}
		}
	}

	if minter.Empty() {
		logger.Error("invalid minter", "minter", minter)
		return
	}

	for _, coin := range coins {
		richlistutils.ApplyBalanceChange(balanceMap, coin.Denom, minter, coin.Amount)
	}
}

func processCosmosBurnEvent(ctx context.Context, q *querier.Querier, logger *slog.Logger, cfg *config.Config, event sdk.Event, balanceMap map[richlistutils.BalanceChangeKey]sdkmath.Int) {
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
			if coins, err = parseCoinsNormalizedDenom(ctx, q, cfg, attr.Value); err != nil {
				logger.Error("failed to parse burned coins", "amount", attr.Value, "error", err)
				return
			}
		}
	}

	if burner.Empty() {
		logger.Error("invalid burner", "burner", burner)
		return
	}

	for _, coin := range coins {
		richlistutils.ApplyBalanceChange(balanceMap, coin.Denom, burner, coin.Amount.Neg())
	}
}

// processCosmosTransferEvent processes a Cosmos transfer event and updates the balance map.
// Module accounts are excluded from balance tracking to avoid tracking treasury and system accounts.
func processCosmosTransferEvent(ctx context.Context, q *querier.Querier, logger *slog.Logger, cfg *config.Config, event sdk.Event, balanceMap map[richlistutils.BalanceChangeKey]sdkmath.Int, moduleAccounts []sdk.AccAddress) {
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
			if coins, err = parseCoinsNormalizedDenom(ctx, q, cfg, attr.Value); err != nil {
				logger.Error("failed to parse transferred coins", "amount", attr.Value, "error", err)
				return
			}
		}
	}

	if recipient.Empty() || sender.Empty() {
		logger.Error("invalid either recipient or sender", "recipient", recipient, "sender", sender)
		return
	}

	for _, coin := range coins {
		if !richlistutils.ContainsAddress(moduleAccounts, sender) {
			richlistutils.ApplyBalanceChange(balanceMap, coin.Denom, sender, coin.Amount.Neg())
		}

		if !richlistutils.ContainsAddress(moduleAccounts, recipient) {
			richlistutils.ApplyBalanceChange(balanceMap, coin.Denom, recipient, coin.Amount)
		}
	}
}
