package evmrichlist

import (
	"context"
	"fmt"
	"log/slog"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/util"
)

func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *orm.Database, startHeight int64, moduleAccounts []sdk.AccAddress) error {
	currentHeight := startHeight

	cfgStartHeight := cfg.GetStartHeight()
	if currentHeight < cfgStartHeight {
		logger.Info("reinitializing rich list", slog.Int64("db_start_height", currentHeight), slog.Int64("config_start_height", cfgStartHeight))
		if err := db.Transaction(func(tx *gorm.DB) error {
			err := richlistutils.InitializeBalances(ctx, logger, tx, cfg, cfgStartHeight)
			return err
		}); err != nil {
			return err
		}
		currentHeight = cfgStartHeight + 1
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
			// Ensure if the next block is indexed in the db
			if _, err := richlistutils.GetCollectedBlock(ctx, dbTx, cfg.GetChainId(), currentHeight); err != nil {
				logger.Error("failed to get block", slog.Any("error", err))
				return err
			}

			// Get the txs for the current height block
			evmTxs, err := GetBlockCollectedEvmTxs(ctx, dbTx, currentHeight)
			if err != nil {
				logger.Error("failed to get evm transactions", slog.Any("error", err))
				return err
			}

			// Process transactions to calculate balance changes
			balanceMap := ProcessEvmBalanceChanges(logger, evmTxs)

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
					balances, err := queryERC20Balances(ctx, cfg.GetChainConfig().JsonRpcUrl, negativeDenom, addresses, currentHeight)
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

			if err := richlistutils.FetchAndUpdateBalances(ctx, logger, dbTx, cfg, moduleAccounts, currentHeight); err != nil {
				return fmt.Errorf("failed to fetch and update balances: %w", err)
			}

			if err := richlistutils.UpdateRichListStatus(ctx, dbTx, currentHeight); err != nil {
				logger.Error("failed to update rich list processed height",
					slog.Int64("height", currentHeight),
					slog.Any("error", err))
				return err
			}

			// Debug: Compare blockchain balances with database balances
			for key, calulatedBalance := range balanceMap {
				// Query balance from blockchain via JSON-RPC
				sdkAddr, err := util.AccAddressFromString(key.Addr)
				if err != nil {
					logger.Error("failed to convert address to sdk address",
						slog.String("address", key.Addr),
						slog.Any("error", err))
					panic(err)
				}
				hexAddr := util.BytesToHexWithPrefix(sdkAddr)
				balances, err := queryERC20Balances(ctx, cfg.GetChainConfig().JsonRpcUrl, key.Denom, []utils.AddressWithID{{HexAddress: hexAddr}}, currentHeight)
				if err != nil {
					logger.Error("failed to query balances",
						slog.String("denom", key.Denom),
						slog.Any("error", err))
					panic(err)
				}

				// Query balance from database
				dbBalanceStr, err := richlistutils.QueryBalance(ctx, dbTx, key.Denom, key.Addr)
				if err != nil {
					logger.Error("failed to query balance from database",
						slog.String("denom", key.Denom),
						slog.String("address", key.Addr),
						slog.Any("error", err))
					panic(err)
				}

				// Parse database balance
				dbBalance, ok := sdkmath.NewIntFromString(dbBalanceStr)
				if !ok {
					panic(fmt.Sprintf("failed to parse database balance: %s", dbBalanceStr))
				}

				// Get blockchain balance for this address
				var blockchainBalance sdkmath.Int
				found := false
				for addrWithID, balance := range balances {
					if addrWithID.HexAddress == hexAddr {
						blockchainBalance = balance
						found = true
						break
					}
				}

				if !found {
					panic(fmt.Sprintf("blockchain balance not found for address %s, denom %s", key.Addr, key.Denom))
				}

				// Compare balances
				if !dbBalance.Equal(blockchainBalance) {
					logger.Error("balance mismatch detected",
						slog.String("denom", key.Denom),
						slog.String("address", key.Addr),
						slog.String("calculated_change", calulatedBalance.String()),
						slog.String("db_balance", dbBalance.String()),
						slog.String("blockchain_balance", blockchainBalance.String()),
						slog.Int64("height", currentHeight))
					panic(fmt.Sprintf("balance mismatch: db=%s, blockchain=%s for address %s, denom %s at height %d",
						dbBalance.String(), blockchainBalance.String(), key.Addr, key.Denom, currentHeight))
				}

				logger.Info("balance verification passed",
					slog.String("denom", key.Denom),
					slog.String("address", key.Addr),
					slog.String("balance", blockchainBalance.String()),
					slog.Int64("height", currentHeight))
			}

			logger.Info("rich list processed height", slog.Int64("height", currentHeight))

			return nil
		}); err != nil {
			return err
		}

		currentHeight += 1
	}
}
