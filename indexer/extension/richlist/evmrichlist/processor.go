package evmrichlist

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	cfg    *config.Config
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *RichList {
	return &RichList{
		cfg:    cfg,
		logger: logger,
	}
}

func (r *RichList) ProcessBalanceChanges(
	_ context.Context,
	_ *querier.Querier,
	logger *slog.Logger,
	txs []rollytypes.CollectedTx,
	_ []sdk.AccAddress,
) map[richlistutils.BalanceChangeKey]sdkmath.Int {
	balanceMap := make(map[richlistutils.BalanceChangeKey]sdkmath.Int)

	richlistutils.ForEachTxEvents(txs, func(events sdk.Events) {
		processEvmTransferEvents(logger, events, balanceMap)
	})

	return balanceMap
}

func (r *RichList) AfterProcess(ctx context.Context, dbTx *gorm.DB, currentHeight int64, negativeDenoms []string, _ *querier.Querier) error {
	if len(negativeDenoms) > 0 {
		r.logger.Info("updating balances for negative denoms", slog.Int("num_denoms", len(negativeDenoms)))

		addresses, err := richlistutils.GetAllAddresses(ctx, dbTx, r.cfg.GetVmType())
		if err != nil {
			r.logger.Error("failed to get all addresses", slog.Any("error", err))
			return err
		}

		for _, negativeDenom := range negativeDenoms {
			balances, err := queryERC20Balances(ctx, r.cfg, negativeDenom, addresses, currentHeight)
			if err != nil {
				r.logger.Error("failed to query balances",
					slog.String("denom", negativeDenom),
					slog.Any("error", err))
				return err
			}

			if err := richlistutils.UpdateBalances(ctx, dbTx, negativeDenom, balances); err != nil {
				r.logger.Error("failed to update balances to database",
					slog.String("denom", negativeDenom),
					slog.Any("error", err))
				return err
			}
		}
	}

	return nil
}

// processEvmTransferEvents processes EVM events and updates the balance map.
// It extracts the EVM log from the event's "log" attribute, parses the JSON-encoded log,
// validates it's an ERC20 Transfer event (by checking the transfer topic), and updates balances
// for both sender (subtract) and receiver (add). The empty address (0x0) is skipped for mint/burn operations.
func processEvmTransferEvents(logger *slog.Logger, events sdk.Events, balanceMap map[richlistutils.BalanceChangeKey]sdkmath.Int) {
	for _, event := range events {
		for _, attr := range event.Attributes {
			if attr.Key != "log" {
				continue
			}

			var evmLog richlistutils.EvmEventLog
			if err := json.Unmarshal([]byte(attr.Value), &evmLog); err != nil {
				logger.Error("failed to unmarshal evm log", "error", err)
				continue
			}

			if len(evmLog.Topics) != 3 || evmLog.Topics[0] != rollytypes.EvmTransferTopic {
				continue
			}

			denom := strings.ToLower(evmLog.Address)
			fromAddr := evmLog.Topics[1]
			toAddr := evmLog.Topics[2]

			amount, ok := richlistutils.ParseHexAmountToSDKInt(evmLog.Data)
			if !ok {
				logger.Error("failed to parse amount from evm log data", "data", evmLog.Data)
				continue
			}

			if fromAccAddr, err := util.AccAddressFromString(fromAddr); fromAddr != rollytypes.EvmEmptyAddress && err == nil {
				richlistutils.ApplyBalanceChange(balanceMap, denom, fromAccAddr, amount.Neg())
			}

			if toAccAddr, err := util.AccAddressFromString(toAddr); toAddr != rollytypes.EvmEmptyAddress && err == nil {
				richlistutils.ApplyBalanceChange(balanceMap, denom, toAccAddr, amount)
			}
		}
	}
}
