package curated

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/config"
	curtypes "github.com/initia-labs/rollytics/curated/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// TODO: it may be replaced with external queue system like RabbitMQ or Kafka
var evmTxQueue = make(chan int64, 100)

func (i *InternalTxIndexer) collect() {
	go func() {
		for {
			// TODO: replace this one to use rabbitmq
			height := <-evmTxQueue
			if height <= 0 {
				continue
			}

			if err := i.collectEvmInternalTxs(i.db, height); err != nil {
				// TODO: replay
				i.logger.Error("failed to collect EVM internal txs", slog.Any("height", height), slog.Any("error", err))
			}
		}

	}()
}

func (i *InternalTxIndexer) collectEvmInternalTxs(db *orm.Database, blockHeight int64) error {
	var g errgroup.Group
	var err error

	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	var callTracerBlockRes *curtypes.CallTracerResponse
	var prestateTracerBlockRes *curtypes.PrestateTracerResponse
	// 1. Get EVM internal transactions for the debug_traceBlock
	g.Go(func() error {
		prestateTracerBlockRes, err = prestateTracerByBlock(i.cfg, client, blockHeight)
		if err != nil {
			return err
		}
		return nil
	})

	g.Go(func() error {
		callTracerBlockRes, err = callTracerByBlock(i.cfg, client, blockHeight)
		if err != nil {
			return err
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		var evmTxs []types.CollectedEvmTx
		if err := tx.Model(&types.CollectedEvmTx{}).
			Where("height = ?", blockHeight).
			Order("sequence ASC").
			Select("hash, height, account_ids").
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
		var internalTxs []types.CollectedEvmInternalTx
		for idx, traceTxRes := range callTracerBlockRes.Result {
			evmTx := evmTxs[idx]
			evmTxAccountIds := evmTx.AccountIds
			accountMap := make(map[int64]any)
			for _, accTd := range evmTxAccountIds {
				accountMap[accTd] = nil
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
				// set account ids for each internal transaction
				subAccIds, err := util.GetOrCreateAccountIds(tx, accounts, true)
				if err != nil {
					return err
				}
				for _, accId := range subAccIds {
					accountMap[accId] = nil
				}
				internalTxs = append(internalTxs, types.CollectedEvmInternalTx{
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
		batchSize := i.cfg.GetDBBatchSize()
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(internalTxs, batchSize).Error; err != nil {
			return err
		}
		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})

	if err != nil {
		// handle serialization error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			i.logger.Info("block already indexed", slog.Int64("height", blockHeight))
			return nil
		}

		return err
	}
	return nil
}

func callTracerByBlock(cfg *config.Config, client *fiber.Client, height int64) (*curtypes.CallTracerResponse, error) {
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

	var res curtypes.CallTracerResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func prestateTracerByBlock(cfg *config.Config, client *fiber.Client, height int64) (*curtypes.PrestateTracerResponse, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "debug_traceBlockByNumber",
		"params": []interface{}{
			fmt.Sprintf("0x%x", height),
			map[string]interface{}{
				"tracer": "prestateTracer",
				"tracerConfig": map[string]interface{}{
					"diffMode": true,
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

	var res curtypes.PrestateTracerResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
