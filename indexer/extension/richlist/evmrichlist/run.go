package evmrichlist

import (
	"context"
	"log/slog"
	"maps"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/orm"
)

func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *orm.Database, startHeight int64) error {
	currentHeight := startHeight

	cfgStartHeight := cfg.GetStartHeight()
	if currentHeight < cfgStartHeight {
		logger.Info("reinitializing rich list", slog.Int64("db_start_height", currentHeight), slog.Int64("config_start_height", cfgStartHeight))
		if err := db.Transaction(func(tx *gorm.DB) error {
			err := richlistutils.InitializeBalances(ctx, logger, tx, cfg.GetChainConfig().RestUrl, cfgStartHeight)
			return err
		}); err != nil {
			return err
		}
		currentHeight = cfgStartHeight + 1
	}

	logger.Info("starting rich list extension", slog.Int64("start_height", currentHeight))
	for {
		if err := db.Transaction(func(tx *gorm.DB) error {
			// Ensure if the next block is indexed in the db
			if _, err := richlistutils.GetCollectedBlock(ctx, tx, cfg.GetChainId(), currentHeight); err != nil {
				logger.Error("failed to get block", slog.Any("error", err))
				return err
			}

			// Get the txs for the current height block
			txs, err := richlistutils.GetBlockCollectedTxs(ctx, tx, currentHeight)
			if err != nil {
				logger.Error("failed to get transactions", slog.Any("error", err))
				return err
			}
			evmTxs, err := GetBlockCollectedEvmTxs(ctx, tx, currentHeight)
			if err != nil {
				logger.Error("failed to get evm transactions", slog.Any("error", err))
				return err
			}

			// Process transactions to calculate balance changes
			balanceMap := richlistutils.ProcessCosmosBalanceChanges(logger, txs)
			evmBalanceMap := ProcessEvmBalanceChanges(logger, evmTxs)
			maps.Copy(balanceMap, evmBalanceMap)

			// Update balance changes to the database
			negativeDenoms, err := richlistutils.UpdateBalanceChanges(ctx, tx, balanceMap)
			if err != nil {
				logger.Error("failed to update balance changes", slog.Any("error", err))
				return err
			}

			// Log warning if any denoms have negative balances
			if len(negativeDenoms) > 0 {
				logger.Info("updating balances for negative denoms", slog.Int("num_denoms", len(negativeDenoms)))

				addresses, err := richlistutils.GetAllAddresses(ctx, tx)
				if err != nil {
					logger.Error("failed to get all addresses", slog.Any("error", err))
					return err
				}

				for _, negativeDenom := range negativeDenoms {
					balances, err := queryERC20Balances(ctx, cfg.GetChainConfig().JsonRpcUrl, negativeDenom, addresses, currentHeight)
					if err != nil {
						logger.Error("failed to query balances",
							slog.String("denom", negativeDenom),
							slog.Any("error", err))
						return err
					}

					if err := richlistutils.UpdateBalances(ctx, tx, negativeDenom, balances); err != nil {
						logger.Error("failed to update balances to database",
							slog.String("denom", negativeDenom),
							slog.Any("error", err))
						return err
					}
				}
			}

			if err := richlistutils.UpdateRichListStatus(ctx, tx, currentHeight); err != nil {
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
