package evm_rich_list

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/rich-list/utils"
	"github.com/initia-labs/rollytics/orm"
)

func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *orm.Database, startHeight int64) error {
	currentHeight := startHeight

	cfgStartHeight := cfg.GetStartHeight()
	if currentHeight < cfgStartHeight {
		if err := db.Transaction(func(tx *gorm.DB) error {
			err := richlistutils.InitializeBalances(ctx, tx, cfg.GetChainConfig().RestUrl, cfgStartHeight)
			return err
		}); err != nil {
			return err
		}
		currentHeight = cfgStartHeight + 1
	}

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

			// Process transactions to calculate balance changes
			balanceMap := richlistutils.ProcessBalanceChanges(logger, txs)

			// Update balance changes to the database
			negativeDenoms, err := richlistutils.UpdateBalanceChanges(ctx, tx, balanceMap)
			if err != nil {
				logger.Error("failed to update balance changes", slog.Any("error", err))
				return err
			}

			// Log warning if any denoms have negative balances
			if len(negativeDenoms) > 0 {
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

			return nil
		}); err != nil {
			return err
		}

		currentHeight += 1
	}
}
