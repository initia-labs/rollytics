package moverichlist

import (
	"context"
	"encoding/json"
	"log/slog"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	richlisttypes "github.com/initia-labs/rollytics/indexer/extension/richlist/types"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	rollytypes "github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/querier"
)

var _ richlisttypes.RichListProcessor = (*RichList)(nil)

type RichList struct {
	logger *slog.Logger
	q      *querier.Querier
}

func New(logger *slog.Logger, q *querier.Querier) *RichList {
	return &RichList{
		logger: logger,
		q:      q,
	}
}

func (r *RichList) ProcessBalanceChanges(
	ctx context.Context,
	txs []rollytypes.CollectedTx,
	moduleAccounts []sdk.AccAddress,
) map[richlistutils.BalanceChangeKey]sdkmath.Int {
	balanceMap := make(map[richlistutils.BalanceChangeKey]sdkmath.Int)

	richlistutils.ForEachTxEvents(txs, func(events sdk.Events) {
		processMoveTransferEvents(ctx, r.q, r.logger, events, balanceMap, moduleAccounts)
	})

	return balanceMap
}

func (r *RichList) AfterProcess(_ context.Context, _ *gorm.DB, _ int64, negativeDenoms []string, _ *querier.Querier) error {
	if len(negativeDenoms) > 0 {
		r.logger.Error("negative denoms found", slog.Int("num_denoms", len(negativeDenoms)))
	}

	return nil
}

// processMoveTransferEvents processes Move VM transfer events (deposit/withdraw) and updates the balance map.
// It handles fungible asset transfers in the Move primary store by matching deposit/withdraw events with their owner events.
// Module accounts are excluded from balance tracking.
func processMoveTransferEvents(ctx context.Context, q *querier.Querier, logger *slog.Logger, events sdk.Events, balanceMap map[richlistutils.BalanceChangeKey]sdkmath.Int, moduleAccounts []sdk.AccAddress) {
	for idx, event := range events {
		if idx == len(events)-1 || event.Type != "move" || len(event.Attributes) < 2 || event.Attributes[0].Key != "type_tag" || len(events[idx+1].Attributes) < 2 || events[idx+1].Attributes[0].Key != "type_tag" {
			continue
		}

		// Support only Fungible Asset in primary store (always following with an owner event)
		// - 0x1::fungible_asset::DepositEvent => 0x1::fungible_asset::DepositOwnerEvent
		// - 0x1::fungible_asset::WithdrawEvent => 0x1::fungible_asset::WithdrawOwnerEvent
		if event.Attributes[0].Value == rollytypes.MoveDepositEventTypeTag && events[idx+1].Attributes[0].Value == rollytypes.MoveDepositOwnerEventTypeTag {
			var depositEvent richlistutils.MoveDepositEvent
			err := json.Unmarshal([]byte(event.Attributes[1].Value), &depositEvent)
			if err != nil {
				logger.Error("failed to unmarshal deposit event", "error", err)
				continue
			}

			var depositOwnerEvent richlistutils.MoveDepositOwnerEvent
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
			denom, err := q.GetMoveDenomByMetadataAddr(ctx, depositEvent.MetadataAddr)
			if err != nil {
				logger.Error("failed to get move denom", "metadataAddr", depositEvent.MetadataAddr, "error", err)
				continue
			}
			amount, ok := sdkmath.NewIntFromString(depositEvent.Amount)
			if !ok {
				logger.Error("failed to parse coin", "coin", depositEvent.Amount, "error", err)
				continue
			}

			if !richlistutils.ContainsAddress(moduleAccounts, recipient) {
				richlistutils.ApplyBalanceChange(balanceMap, denom, recipient, amount)
			}
		}

		if event.Attributes[0].Value == rollytypes.MoveWithdrawEventTypeTag && events[idx+1].Attributes[0].Value == rollytypes.MoveWithdrawOwnerEventTypeTag {
			var withdrawEvent richlistutils.MoveWithdrawEvent
			err := json.Unmarshal([]byte(event.Attributes[1].Value), &withdrawEvent)
			if err != nil {
				logger.Error("failed to unmarshal withdraw event", "error", err)
				continue
			}

			var withdrawOwnerEvent richlistutils.MoveWithdrawOwnerEvent
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
			denom, err := q.GetMoveDenomByMetadataAddr(ctx, withdrawEvent.MetadataAddr)
			if err != nil {
				logger.Error("failed to get move denom", "metadataAddr", withdrawEvent.MetadataAddr, "error", err)
				continue
			}
			amount, ok := sdkmath.NewIntFromString(withdrawEvent.Amount)
			if !ok {
				logger.Error("failed to parse coin", "coin", withdrawEvent.Amount)
				continue
			}

			if !richlistutils.ContainsAddress(moduleAccounts, sender) {
				richlistutils.ApplyBalanceChange(balanceMap, denom, sender, amount.Neg())
			}
		}
	}
}
