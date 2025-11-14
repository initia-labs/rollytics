package evmrichlist

import (
	"context"
	"fmt"
	"log/slog"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/getsentry/sentry-go"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/sentry_integration"
	"github.com/initia-labs/rollytics/util"
)

func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger, db *orm.Database, startHeight int64, moduleAccounts []sdk.AccAddress, requireInit bool) error {
	currentHeight := startHeight

	if requireInit {
		logger.Info("reinitializing rich list", slog.Int64("start_height", currentHeight))
		if err := db.Transaction(func(tx *gorm.DB) error {
			err := richlistutils.InitializeBalances(ctx, logger, tx, cfg, currentHeight)
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
			balanceMap := richlistutils.ProcessCosmosBalanceChanges(logger, cfg, cosmosTxs, moduleAccounts)

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

			// Debug: Compare blockchain balances with database balances for addresses with balance changes.
			// Queries on-chain state via JSON-RPC and batch-corrects any detected mismatches.
			blockchainBalancesByDenom := make(map[string]map[richlistutils.AddressWithID]sdkmath.Int)
			for key, amountChange := range balanceMap {
				// Query balance from blockchain via JSON-RPC for verification
				if sdkAddr, err := util.AccAddressFromString(key.Addr); err == nil {
					hexAddr := util.BytesToHexWithPrefix(sdkAddr)
					balances, err := queryERC20Balances(ctx, cfg.GetChainConfig().JsonRpcUrl, key.Denom, []richlistutils.AddressWithID{{HexAddress: hexAddr}}, currentHeight)
					if err != nil {
						logger.Error("failed to query balances",
							slog.String("denom", key.Denom),
							slog.Any("error", err))
						return err
					}

					dbBalanceStr, err := richlistutils.QueryBalance(ctx, dbTx, key.Denom, key.Addr)
					if err != nil {
						return err
					}

					dbBalance, ok := sdkmath.NewIntFromString(dbBalanceStr.Amount)
					if !ok {
						return fmt.Errorf("failed to parse database balance: %s", dbBalanceStr.Amount)
					}

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
						return fmt.Errorf("blockchain balance not found for address %s, denom %s", key.Addr, key.Denom)
					}

					// Compare balances
					if !dbBalance.Equal(blockchainBalance) {
						logger.Error("balance mismatch detected",
							slog.String("denom", key.Denom),
							slog.String("address", key.Addr),
							slog.String("balance_change", amountChange.String()),
							slog.String("db_balance", dbBalance.String()),
							slog.String("blockchain_balance", blockchainBalance.String()),
							slog.Int64("height", currentHeight))

						if blockchainBalancesByDenom[key.Denom] == nil {
							blockchainBalancesByDenom[key.Denom] = make(map[richlistutils.AddressWithID]sdkmath.Int)
						}
						blockchainBalancesByDenom[key.Denom][richlistutils.NewAddressWithID(sdkAddr, dbBalanceStr.Id)] = blockchainBalance

						// Send error to Sentry with all structured fields
						errMsg := fmt.Errorf("balance mismatch detected: db=%s, blockchain=%s for address %s, denom %s at height %d, balance change=%s",
							dbBalance.String(), blockchainBalance.String(), key.Addr, key.Denom, currentHeight, amountChange.String())

						// TODO: remove
						panic(errMsg)

						sentry_integration.CaptureExceptionWithContext(errMsg, sentry.LevelWarning,
							map[string]string{
								"denom":   key.Denom,
								"address": key.Addr,
								"height":  fmt.Sprintf("%d", currentHeight),
							},
							map[string]any{
								"balance_change":     amountChange.String(),
								"db_balance":         dbBalance.String(),
								"blockchain_balance": blockchainBalance.String(),
							})
					}
				}
			}

			// Batch update all mismatches grouped by denom
			if len(blockchainBalancesByDenom) > 0 {
				// Update balances once per denom
				for denom, balanceMapToUpdate := range blockchainBalancesByDenom {
					if err := richlistutils.UpdateBalances(ctx, dbTx, denom, balanceMapToUpdate); err != nil {
						logger.Error("failed to update balances in database",
							slog.String("denom", denom),
							slog.Any("error", err))
						return err
					}
				}

				logger.Info("balance mismatches corrected", slog.Int("num_blockchain_balances", len(blockchainBalancesByDenom)))
			}

			logger.Info("rich list processed height", slog.Int64("height", currentHeight))

			return nil
		}); err != nil {
			return err
		}

		currentHeight += 1
	}
}
