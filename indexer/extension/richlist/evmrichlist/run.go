package evmrichlist

import (
	"context"
	"log/slog"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/util/querier"
)

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

			// Log warning if any denoms have negative balances
			if len(negativeDenoms) > 0 {
				logger.Info("updating balances for negative denoms", slog.Int("num_denoms", len(negativeDenoms)))

				addresses, err := richlistutils.GetAllAddresses(ctx, dbTx, cfg.GetVmType())
				if err != nil {
					logger.Error("failed to get all addresses", slog.Any("error", err))
					return err
				}

				for _, negativeDenom := range negativeDenoms {
					balances, err := queryERC20Balances(ctx, cfg, negativeDenom, addresses, currentHeight)
					if err != nil {
						logger.Error("failed to query balances",
							slog.String("denom", negativeDenom),
							slog.Any("error", err))
						return err
					}

					if err := richlistutils.UpdateBalances(ctx, dbTx, negativeDenom, balances); err != nil {
						logger.Error("failed to update balances to database",
							slog.String("denom", negativeDenom),
							slog.Any("error", err))
						return err
					}
				}

			}

			// NOTE: EVM events don't care module account
			// if err := richlistutils.FetchAndUpdateBalances(ctx, logger, dbTx, cfg, moduleAccounts, currentHeight); err != nil {
			// 	return fmt.Errorf("failed to fetch and update balances: %w", err)
			// }

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
