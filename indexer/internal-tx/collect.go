package internal_tx

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"

	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type InternalTxResult struct {
	Height    int64
	CallTrace *DebugCallTraceBlockResponse
}

func (i *Indexer) collect(heights []int64) {
	var (
		g        errgroup.Group
		scrapped = make(map[int64]*InternalTxResult)
		mu       sync.Mutex
	)
	// 1. Scrape internal transactions
	client := fiber.AcquireClient()
	for _, h := range heights {
		g.Go(func() error {
			internalTx, err := i.scrapInternalTxs(client, h)
			if err != nil {
				i.logger.Error("failed to scrap internal txs", slog.Int64("height", h), slog.Any("error", err))
				return err
			}

			i.logger.Info("scraped internal txs", slog.Int64("height", h))
			mu.Lock()
			scrapped[internalTx.Height] = internalTx
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		panic(err)
	}
	fiber.ReleaseClient(client)

	// 2. Collect internal transactions
	for _, h := range heights {
		internalTx := scrapped[h]
		if err := i.CollectInternalTxs(i.db, internalTx); err != nil {
			i.logger.Error("failed to collect internal txs", slog.Int64("height", internalTx.Height), slog.Any("error", err))
			panic(err)
		}
	}
}

// Get EVM internal transactions for the debug_traceBlock
func (i *Indexer) scrapInternalTxs(client *fiber.Client, height int64) (*InternalTxResult, error) {
	callTraceRes, err := TraceCallByBlock(i.cfg, client, height)
	if err != nil {
		return nil, err
	}
	return &InternalTxResult{
		Height:    int64(height),
		CallTrace: callTraceRes,
	}, nil
}

func (i *Indexer) CollectInternalTxs(db *orm.Database, internalTx *InternalTxResult) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		seqInfo, err := indexerutil.GetSeqInfo("evm_internal_tx", tx)
		if err != nil {
			return err
		}
		var evmTxs []types.CollectedEvmTx
		if err := tx.Model(&types.CollectedEvmTx{}).
			Where("height = ?", internalTx.Height).
			Order("sequence ASC").
			Select("hash, height, account_ids").
			Find(&evmTxs).Error; err != nil {
			return err
		}

		if len(internalTx.CallTrace.Result) != len(evmTxs) {
			return fmt.Errorf("number of internal transactions (callTrace: %d, evmTxs: %d) at height %d does not match",
				len(internalTx.CallTrace.Result), len(evmTxs), internalTx.Height)
		}

		var allInternalTxs []types.CollectedEvmInternalTx
		for idx, trace := range internalTx.CallTrace.Result {
			evmTx := evmTxs[idx]
			height, txHash, evmTxAccountIds := evmTx.Height, evmTx.Hash, evmTx.AccountIds
			txInfo := &InternalTxInfo{
				Height: height,
				Hash:   txHash,
				Index:  int64(0),
			}
			accountMap := make(map[int64]any)
			for _, accId := range evmTxAccountIds {
				accountMap[accId] = nil
			}
			// Process the top-level call and sub-calls
			// 1. Top-level call
			topLevelCall := InternalTransaction{
				Type:    trace.Result.Type,
				From:    trace.Result.From,
				To:      trace.Result.To,
				Value:   trace.Result.Value,
				Gas:     trace.Result.Gas,
				GasUsed: trace.Result.GasUsed,
				Input:   trace.Result.Input,
				Output:  "", // Top-level calls don't have output
			}

			topLevelTx, err := processInternalCall(
				tx,
				txInfo,
				&topLevelCall,
				&seqInfo,
				accountMap,
			)
			if err != nil {
				return err
			}
			allInternalTxs = append(allInternalTxs, *topLevelTx)

			// 2. Sub-calls
			for subIdx, call := range trace.Result.Calls {
				txInfo.Index = int64(subIdx + 1)
				subTx, err := processInternalCall(
					tx,
					txInfo,
					&call,
					&seqInfo,
					accountMap,
				)
				if err != nil {
					return err
				}
				allInternalTxs = append(allInternalTxs, *subTx)
			}

			accountIds := make([]int64, 0, len(accountMap))
			for accId := range accountMap {
				accountIds = append(accountIds, accId)
			}
		}
		batchSize := i.cfg.GetDBBatchSize()
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(allInternalTxs, batchSize).Error; err != nil {
			i.logger.Error("failed to create internal txs batch", slog.Int64("height", internalTx.Height), slog.Any("error", err))
			return err
		}

		// Update the sequence info
		if err := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error; err != nil {
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
			i.logger.Info("block already indexed", slog.Int64("height", internalTx.Height))
			return nil
		}

		return err
	}
	return nil
}
