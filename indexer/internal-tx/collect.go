package internal_tx

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

const (
	numWorkers = 10
)

type InternalTxResult struct {
	Height       int64
	CallTraceRes *CallTracerResponse
	PrestateRes  *PrestateTracerResponse
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

	// Start workers to scrap internal transactions in parallel
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

	// Write results to the database sequentially in heights order
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

			for _, result := range pendingItxs {
				if err := i.CollectInternalTxs(i.db, result); err != nil {
					i.logger.Error("failed to collect internal txs", slog.Int64("height", result.Height), slog.Any("error", err))
					errChan <- err
					return
				}
				i.logger.Info("collected internal txs", slog.Int64("height", result.Height))
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
		return nil, err
	}
	return &InternalTxResult{
		Height:       int64(height),
		CallTraceRes: callTraceRes,
		PrestateRes:  prestateTraceRes,
	}, nil
}

// Iterate over the internal calls of transaction
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

		if len(internalTx.CallTraceRes.Result) != len(evmTxs) || len(internalTx.PrestateRes.Result) != len(evmTxs) {
			return fmt.Errorf("number of internal transactions (callTrace: %d, prestateTrace: %d, evmTxs: %d) at height %d does not match",
				len(internalTx.CallTraceRes.Result), len(internalTx.PrestateRes.Result), len(evmTxs), internalTx.Height)
		}

		var internalTxs []types.CollectedEvmInternalTx
		for idx, traceTxWrapper := range internalTx.CallTraceRes.Result {
			traceTxRes := traceTxWrapper.Result
			prestateTracing := internalTx.PrestateRes.Result[idx]
			evmTx := evmTxs[idx]
			evmTxAccountIds := evmTx.AccountIds
			accountMap := make(map[int64]any)
			for _, accTd := range evmTxAccountIds {
				accountMap[accTd] = nil
			}
			for subIdx, internalTx := range traceTxRes.Calls {
				gas := int64(0)
				if internalTx.Gas != "" {
					var err error
					gas, err = strconv.ParseInt(internalTx.Gas, 0, 64)
					if err != nil {
						return err
					}
				}

				gasUsed := int64(0)
				if internalTx.GasUsed != "" {
					var err error
					gasUsed, err = strconv.ParseInt(internalTx.GasUsed, 0, 64)
					if err != nil {
						return err
					}
				}

				value := int64(0)
				if internalTx.Value != "" {
					var err error
					value, err = strconv.ParseInt(internalTx.Value, 0, 64)
					if err != nil {
						return err
					}
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
				Update("account_ids", pq.Array(accountIds)).Error; err != nil {
				return err
			}

		}
		batchSize := i.cfg.GetDBBatchSize()
		if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(internalTxs, batchSize).Error; err != nil {
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
