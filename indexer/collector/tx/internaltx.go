package tx

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// TODO: it may be replaced with external queue system like RabbitMQ or Kafka
var evmTxQueue = make(chan int64, 100)

func processEvmInternalTxs(cfg *config.Config, client *fiber.Client, db *gorm.DB) {
	if cfg.GetVmType() != types.EVM || !cfg.GetChainConfig().InternalTx {
		return
	}

	go func() {
		blockHeight := <-evmTxQueue

		var g errgroup.Group
		var err error
		var callTracerBlockRes *CallTracerResponse
		var prestateTracerBlockRes *PrestateTracerResponse
		// 1. Get EVM internal transactions for the debug_traceBlock
		g.Go(func() error {
			prestateTracerBlockRes, err = prestateTracerByBlock(cfg, client, blockHeight)
			if err != nil {
				// error handling
				return err
			}
			return nil
		})

		g.Go(func() error {
			callTracerBlockRes, err = callTracerByBlock(cfg, client, blockHeight)
			if err != nil {
				// error handling
				return err
			}
			return nil
		})

		if err := g.Wait(); err != nil {
			// error handling: failed to get EVM internal transactions
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			var evmTxs []types.CollectedEvmTx
			if err := tx.Model(&types.CollectedEvmTx{}).
				Where("height = ?", blockHeight).
				Order("sequence ASC").
				Select("hash,height, account_ids").
				Find(&evmTxs).Error; err != nil {
				// error handling
				return err
			}

			if len(callTracerBlockRes.Result) != len(evmTxs) || len(prestateTracerBlockRes.Result) != len(evmTxs) {
				// error handling: number of internal transactions does not match the number of EVM transactions
				return fmt.Errorf("number of internal transactions (%d) does not match the number of EVM transactions (%d) at height %d",
					len(callTracerBlockRes.Result), len(evmTxs), blockHeight)
			}

			// 2. Iterate over the internal calls of transaction
			var evmInternalTxs []types.CollectedEvmInternalTx
			for idx, traceTxRes := range callTracerBlockRes.Result {
				evmTx := evmTxs[idx]
				originAccountIds := evmTx.AccountIds
				accountMap := make(map[int64]interface{})
				for _, accountId := range originAccountIds {
					accountMap[accountId] = nil
				}
				for subIdx, internalTx := range traceTxRes.Calls {
					gas, err := strconv.ParseInt(internalTx.Gas, 0, 64)
					if err != nil {
						// error handling: failed to parse gas
						return err
					}
					gasUsed, err := strconv.ParseInt(internalTx.GasUsed, 0, 64)
					if err != nil {
						// error handling: failed to parse gasUsed
						return err
					}
					value, err := strconv.ParseInt(internalTx.Value, 0, 64)
					if err != nil {
						// error handling: failed to parse value
						return err
					}
					accounts, err := grepAddressesFromEvmInternalTx(internalTx)
					if err != nil {
						return err
					}

					subAccIds, err := util.GetOrCreateAccountIds(db, accounts, true)
					if err != nil {
						return err
					}
					for _, accId := range subAccIds {
						accountMap[accId] = nil
					}
					evmInternalTxs = append(evmInternalTxs, types.CollectedEvmInternalTx{
						Height:     evmTx.Height,
						Hash:       evmTx.Hash,
						Index:      int64(subIdx),
						Type:       internalTx.Type,
						From:       internalTx.From,
						To:         internalTx.To,
						Input:      internalTx.Input,
						Output:     internalTx.Output,
						Value:      value,
						Gas:        gas,
						GasUsed:    gasUsed,
						AccountIds: subAccIds,
					})

				}
				accountIds := make([]int64, 0, len(accountMap))
				for accId := range accountMap {
					accountIds = append(accountIds, accId)
				}
				if err := tx.Model(&types.CollectedEvmTx{}).
					Where("hash = ? AND height = ?", evmTx.Hash, evmTx.Height).
					Update("account_ids = ?", accountIds).Error; err != nil {
					// error handling
					return err
				}

			}
			batchSize := cfg.GetDBBatchSize()
			if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(evmInternalTxs, batchSize).Error; err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			// error handling: failed to process EVM internal transactions
			return
		}

	}()
}

func callTracerByBlock(cfg *config.Config, client *fiber.Client, height int64) (*CallTracerResponse, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "debug_traceBlockByNumber",
		"params":  []string{fmt.Sprintf("0x%x", height), `{"tracer": "callTracer"}`},
		"id":      1,
	}
	headers := map[string]string{"Content-Type": "application/json"}

	body, err := util.Post(client, cfg.GetCoolingDuration(), cfg.GetChainConfig().JsonRpcUrl, "", payload, headers)
	if err != nil {
		return nil, err
	}

	var res CallTracerResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func prestateTracerByBlock(cfg *config.Config, client *fiber.Client, height int64) (*PrestateTracerResponse, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "debug_traceBlockByNumber",
		"params": []interface{}{
			fmt.Sprintf("0x%x", height),
			map[string]interface{}{
				"tracer": "prestateTracer",
				"tracerConfig": map[string]interface{}{
					"diffMode":    true,
				},
			},
		},
		"id": 1,
	}
	headers := map[string]string{"Content-Type": "application/json"}

	body, err := util.Post(client, cfg.GetCoolingDuration(), cfg.GetChainConfig().JsonRpcUrl, "", payload, headers)
	if err != nil {
		return nil, err
	}

	var res PrestateTracerResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
