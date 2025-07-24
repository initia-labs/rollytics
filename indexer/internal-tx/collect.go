package internal_tx

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"

	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

func (i *Indexer) collect(heightChan <-chan int64) {
	for height := range heightChan {
		// 1. Scrap internal transactions from the EVM block
		callTraceRes, prestateRes, err := i.scrapInternalTxs(height)
		if err != nil {
			i.logger.Error("failed to scrap internal transactions", slog.Int64("height", height), slog.Any("error", err))
			panic(err)
		}
		// 2. Collect internal transactions
		if err := i.CollectInternalTxs(i.db, height, callTraceRes, prestateRes); err != nil {
			i.logger.Error("failed to collect internal transactions", slog.Int64("height", height), slog.Any("error", err))
			panic(err)
		}
		time.Sleep(i.cfg.GetCoolingDuration())
	}
}

// Get EVM internal transactions for the debug_traceBlock
func (i *Indexer) scrapInternalTxs(height int64) (*CallTracerResponse, *PrestateTracerResponse, error) {
	var (
		g                errgroup.Group
		err              error
		callTraceRes     *CallTracerResponse
		prestateTraceRes *PrestateTracerResponse
	)

	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	g.Go(func() error {
		callTraceRes, err = TraceCallByBlock(i.cfg, client, height)
		if err != nil {
			return err
		}
		return nil
	})

	g.Go(func() error {
		prestateTraceRes, err = TraceStateByBlock(i.cfg, client, height)
		if err != nil {
			return err
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	return callTraceRes, prestateTraceRes, nil
}

// Iterate over the internal calls of transaction
func (i *Indexer) CollectInternalTxs(db *orm.Database, height int64, callTraceRes *CallTracerResponse, prestateTraceRes *PrestateTracerResponse) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		seqInfo, err := indexerutil.GetSeqInfo("evm_internal_tx", tx)
		if err != nil {
			return err
		}
		var evmTxs []types.CollectedEvmTx
		if err := tx.Model(&types.CollectedEvmTx{}).
			Where("height = ?", height).
			Order("sequence ASC").
			Select("hash, height, account_ids").
			Find(&evmTxs).Error; err != nil {
			return err
		}

		if len(callTraceRes.Result) != len(evmTxs) || len(prestateTraceRes.Result) != len(evmTxs) {
			return fmt.Errorf("number of internal transactions (callTrace: %d, prestateTrace: %d, evmTxs: %d) at height %d does not match",
				len(callTraceRes.Result), len(prestateTraceRes.Result), len(evmTxs), height)
		}

		var internalTxs []types.CollectedEvmInternalTx
		for idx, traceTxRes := range callTraceRes.Result {
			prestateTracing := prestateTraceRes.Result[idx]
			evmTx := evmTxs[idx]
			evmTxAccountIds := evmTx.AccountIds
			accountMap := make(map[int64]any)
			for _, accTd := range evmTxAccountIds {
				accountMap[accTd] = nil
			}
			for subIdx, internalTx := range traceTxRes.Calls {
				gas, err := strconv.ParseInt(internalTx.Gas, 0, 64)
				if err != nil {
					return err
				}
				gasUsed, err := strconv.ParseInt(internalTx.GasUsed, 0, 64)
				if err != nil {
					return err
				}
				value, err := strconv.ParseInt(internalTx.Value, 0, 64)
				if err != nil {
					return err
				}
				accounts, err := GrepAddressesFromEvmInternalTx(internalTx)
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
				seqInfo.Sequence++

				internalTxs = append(internalTxs, types.CollectedEvmInternalTx{
					Height:     evmTx.Height,
					Hash:       evmTx.Hash,
					Sequence:   int64(seqInfo.Sequence),
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
					PreState:   prestateTracing.Result.Pre,
					PostState:  prestateTracing.Result.Post,
				})

			}
			accountIds := make([]int64, 0, len(accountMap))
			for accId := range accountMap {
				accountIds = append(accountIds, accId)
			}
			if err := tx.Model(&types.CollectedEvmTx{}).
				Where("hash = ? AND height = ?", evmTx.Hash, evmTx.Height).
				Update("account_ids", accountIds).Error; err != nil {
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
		// handle intended serialization error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			i.logger.Info("block already indexed", slog.Int64("height", height))
			return nil
		}

		return err
	}
	return nil
}
