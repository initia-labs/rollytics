package internal_tx

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

const (
	numWorkers = 10
)

type InternalTxResult struct {
	Height    int64
	CallTrace *DebugCallTraceBlockResponse
}

func (i *Indexer) collect(heightChan <-chan int64, startHeight int64) {
	var (
		wg             sync.WaitGroup
		results        = make(chan *InternalTxResult, 100)
		errChan        = make(chan error, 1)
		pendingMap     = make(map[int64]*InternalTxResult)
		expectedHeight = startHeight
		mu             sync.Mutex
	)

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for height := range heightChan {
				internalTx, err := i.scrapInternalTxs(height)
				if err != nil {
					i.logger.Error("failed to scrap internal txs", slog.Int64("height", height), slog.Any("error", err))
					errChan <- err
					return
				}
				i.logger.Info("scraped internal txs", slog.Int64("height", height))
				results <- internalTx
				time.Sleep(i.cfg.GetCoolingDuration())
			}
		}()
	}

	go func() {
		for res := range results {
			mu.Lock()
			pendingMap[res.Height] = res

			var pendingItxs []*InternalTxResult
			for {
				if pendingItx, exists := pendingMap[expectedHeight]; exists {
					delete(pendingMap, expectedHeight)
					pendingItxs = append(pendingItxs, pendingItx)
					expectedHeight++
				} else {
					break
				}
			}
			if len(pendingMap) > 1000 {
				i.logger.Warn("too many pending results", slog.Int("pending", len(pendingMap)), slog.Int64("expectedHeight", expectedHeight))
			}
			mu.Unlock()

			for _, internalTx := range pendingItxs {
				if err := i.CollectInternalTxs(i.db, internalTx); err != nil {
					i.logger.Error("failed to collect internal txs", slog.Int64("height", internalTx.Height), slog.Any("error", err))
					errChan <- err
					return
				}
				i.logger.Info("collected internal txs", slog.Int64("height", internalTx.Height))
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	if err := <-errChan; err != nil {
		panic(err)
	}
}

// Get EVM internal transactions for the debug_traceBlock
func (i *Indexer) scrapInternalTxs(height int64) (*InternalTxResult, error) {
	var (
		g            errgroup.Group
		err          error
		callTraceRes *DebugCallTraceBlockResponse
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

	if err := g.Wait(); err != nil {
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

			// If the new account IDs are found from the internal transactions, update the evm_tx record
			if err := tx.Model(&types.CollectedEvmTx{}).
				Where("hash = ? AND height = ?", txHash, height).
				Update("account_ids", pq.Array(accountIds)).Error; err != nil {
				return err
			}
		}
		batchSize := i.cfg.GetDBBatchSize()
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(allInternalTxs, batchSize).Error; err != nil {
			i.logger.Error("failed to create internal txs batch", slog.Int64("height", internalTx.Height), slog.Any("error", err))
			return err
		}

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
