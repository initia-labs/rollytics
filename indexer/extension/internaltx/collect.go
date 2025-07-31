package internaltx

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

type InternalTxResult struct {
	Height    int64
	CallTrace *DebugCallTraceBlockResponse
}

func (i *InternalTxExtension) collect(heights []int64) error {
	var (
		g       errgroup.Group
		scraped = make(map[int64]*InternalTxResult)
		mu      sync.Mutex
	)

	// 1. Scrape internal transactions
	for _, height := range heights {
		h := height
		g.Go(func() error {
			client := fiber.AcquireClient()
			defer fiber.ReleaseClient(client)
			internalTx, err := i.scrapeInternalTx(client, h)
			if err != nil {
				i.logger.Error("failed to scrape internal tx", slog.Int64("height", h), slog.Any("error", err))
				return err
			}

			i.logger.Info("scraped internal txs", slog.Int64("height", h))
			mu.Lock()
			scraped[internalTx.Height] = internalTx
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// 2. Collect internal transactions
	for _, height := range heights {
		if err := i.CollectInternalTxs(i.db, scraped[height]); err != nil {
			i.logger.Error("failed to collect internal txs", slog.Int64("height", height), slog.Any("error", err))
			return err
		}
	}

	return nil
}

// Get EVM internal transactions for the debug_traceBlock
func (i *InternalTxExtension) scrapeInternalTx(client *fiber.Client, height int64) (*InternalTxResult, error) {
	callTraceRes, err := TraceCallByBlock(i.cfg, client, height)
	if err != nil {
		return nil, err
	}

	return &InternalTxResult{
		Height:    height,
		CallTrace: callTraceRes,
	}, nil
}

func (i *InternalTxExtension) CollectInternalTxs(db *orm.Database, internalTx *InternalTxResult) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		seqInfo, err := indexerutil.GetSeqInfo(types.SeqInfoEvmInternalTx, tx)
		if err != nil {
			return err
		}
		var evmTxs []types.CollectedEvmTx
		if err := tx.Model(&types.CollectedEvmTx{}).
			Select("hash, height, account_ids").
			Where("height = ?", internalTx.Height).
			Order("sequence ASC").
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
			height, txHash := evmTx.Height, evmTx.Hash

			topLevelCall := InternalTransaction{
				Type:    trace.Result.Type,
				From:    trace.Result.From,
				To:      trace.Result.To,
				Value:   trace.Result.Value,
				Gas:     trace.Result.Gas,
				GasUsed: trace.Result.GasUsed,
				Input:   trace.Result.Input,
				Output:  "", // Top-level calls don't have output
				Calls:   trace.Result.Calls,
			}

			txInfo := &InternalTxInfo{
				Height:      height,
				Hash:        txHash,
				Index:       0,  // Top-level starts at index 0
				ParentIndex: -1, // Top-level has no parent
			}

			txResults, err := processInternalCall(tx, txInfo, &topLevelCall, &seqInfo)
			if err != nil {
				return err
			}
			allInternalTxs = append(allInternalTxs, txResults...)
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
		Isolation: sql.LevelRepeatableRead,
	})

	if err != nil {
		// handle intended serialization error
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			i.logger.Info("evm internal tx already indexed", slog.Int64("height", internalTx.Height))
			return nil
		}

		return err
	}

	return nil
}
