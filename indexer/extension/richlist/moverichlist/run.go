package moverichlist

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/querier"
)

func sanityCheckBalances(
	ctx context.Context,
	querier *querier.Querier,
	dbTx *gorm.DB,
	balanceMap map[richlistutils.BalanceChangeKey]sdkmath.Int,
	height int64,
) error {
	if len(balanceMap) == 0 {
		return nil
	}

	addressDenoms := make(map[string]map[string]struct{})
	for key := range balanceMap {
		if addressDenoms[key.Addr] == nil {
			addressDenoms[key.Addr] = make(map[string]struct{})
		}
		addressDenoms[key.Addr][key.Denom] = struct{}{}
		// TODO: Remove this
		fmt.Println(key.Addr, key.Denom, balanceMap[key])
	}

	for addr, denomSet := range addressDenoms {
		accAddr, err := util.AccAddressFromString(addr)
		if err != nil {
			return fmt.Errorf("failed to parse address %s: %w", addr, err)
		}

		onChainBalances, err := querier.GetAllBalances(ctx, accAddr, height)
		if err != nil {
			return fmt.Errorf("failed to fetch on-chain balances for %s: %w", addr, err)
		}

		// TODO: Remove this
		fmt.Println(onChainBalances)
		onChainMap := make(map[string]sdkmath.Int, len(onChainBalances))
		for _, coin := range onChainBalances {
			if coin.Amount.IsZero() {
				continue
			}
			onChainMap[coin.Denom] = coin.Amount
		}

		for denom := range denomSet {
			onChainAmount, ok := onChainMap[denom]
			if !ok {
				// Missing denom implies zero balance on-chain.
				onChainAmount = sdkmath.ZeroInt()
			}
			dbAmount := sdkmath.ZeroInt()

			dbBalance, err := richlistutils.QueryBalance(ctx, dbTx, denom, addr)
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("failed to query db balance for %s (%s): %w", addr, denom, err)
				}
			} else {
				amount, ok := sdkmath.NewIntFromString(dbBalance.Amount)
				if !ok {
					return fmt.Errorf("failed to parse db amount for %s (%s): %s", addr, denom, dbBalance.Amount)
				}
				dbAmount = amount
			}

			if !onChainAmount.Equal(dbAmount) {
				panic(fmt.Sprintf(
					"rich list sanity check failed: address=%s denom=%s height=%d chain=%s db=%s",
					addr,
					denom,
					height,
					onChainAmount.String(),
					dbAmount.String(),
				))
			}
		}
	}

	return nil
}

func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *orm.Database, startHeight int64, moduleAccounts []sdk.AccAddress, requireInit bool) error {
	currentHeight := startHeight
	querier := querier.NewQuerier(cfg.GetChainConfig())
	if requireInit {
		logger.Info("reinitializing rich list", slog.Int64("start_height", currentHeight))
		if err := db.Transaction(func(tx *gorm.DB) error {
			err := richlistutils.InitializeBalances(ctx, querier, logger, tx, cfg, currentHeight)
			return err
		}); err != nil {
			return err
		}
		currentHeight = currentHeight + 1
	}

	logger.Info("starting rich list extension", slog.Int64("start_height", currentHeight))
	for {
		// context cancellation check
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := db.Transaction(func(dbTx *gorm.DB) error {
			// Verify that the block at current height exists in the database
			if _, err := richlistutils.GetCollectedBlock(ctx, dbTx, cfg.GetChainId(), currentHeight); err != nil {
				logger.Error("failed to get block", slog.Any("error", err))
				return err
			}

			cosmosTxs, err := richlistutils.GetBlockCollectedTxs(ctx, dbTx, currentHeight)
			if err != nil {
				logger.Error("failed to get cosmos transactions", slog.Any("error", err))
				return err
			}

			// Process cosmos transactions to calculate balance changes
			balanceMap := richlistutils.ProcessBalanceChanges(ctx, querier, logger, cfg, cosmosTxs, moduleAccounts)

			// Update balance changes to the database
			negativeDenoms, err := richlistutils.UpdateBalanceChanges(ctx, dbTx, balanceMap)
			if err != nil {
				logger.Error("failed to update balance changes", slog.Any("error", err))
				return err
			}

			if err := sanityCheckBalances(ctx, querier, dbTx, balanceMap, currentHeight); err != nil {
				logger.Error("failed rich list sanity check", slog.Int64("height", currentHeight), slog.Any("error", err))
				return err
			}

			// Log warning if any denoms have negative balances
			if len(negativeDenoms) > 0 {
				logger.Error("negative denoms found", slog.Int("num_denoms", len(negativeDenoms)))
			}

			if err := richlistutils.UpdateRichListStatus(ctx, dbTx, currentHeight); err != nil {
				logger.Error("failed to update rich list processed height",
					slog.Int64("height", currentHeight),
					slog.Any("error", err))
				return err
			}

			logger.Info("rich list processed height", slog.Int64("height", currentHeight))

			return nil
		}); err != nil {
			return err
		}

		currentHeight += 1
	}
}
