package internaltx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/sentry_integration"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

type InternalTxResult struct {
	Height    int64
	CallTrace *DebugCallTraceBlockResponse
}

func (i *InternalTxExtension) collect(ctx context.Context, heights []int64) error {
	// Use context-aware errgroup for cancellation support
	g, gCtx := errgroup.WithContext(ctx)
	scraped := make(map[int64]*InternalTxResult)
	mu := sync.Mutex{}

	// 1. Scrape internal transactions with context
	for _, height := range heights {
		h := height
		g.Go(func() error {
			// Check context before starting
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			span, _ := sentry_integration.StartSentrySpan(gCtx, "scrapeInternalTx", "Scraping internal transactions for height "+strconv.FormatInt(h, 10))
			defer span.Finish()

			client := fiber.AcquireClient()
			defer fiber.ReleaseClient(client)

			// TODO: Update scrapeInternalTx to support context for HTTP timeouts
			internalTx, err := i.scrapeInternalTx(client, h)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					i.logger.Info("scraping cancelled", slog.Int64("height", h))
					return err
				}
				i.logger.Error("failed to scrape internal tx",
					slog.Int64("height", h),
					slog.Any("error", err))
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

	// 2. Collect internal transactions with context check
	for _, height := range heights {
		// Check context before each DB operation
		select {
		case <-ctx.Done():
			i.logger.Info("collection cancelled during DB save", slog.Int64("height", height))
			return ctx.Err()
		default:
		}

		err := func() error {
			span, _ := sentry_integration.StartSentrySpan(ctx, "collectInternalTxs", "Collecting internal transactions for height "+strconv.FormatInt(height, 10))
			defer span.Finish()

			// TODO: Update CollectInternalTxs to use context for DB operations
			if err := i.CollectInternalTxs(i.db, scraped[height]); err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
				i.logger.Error("failed to collect internal txs",
					slog.Int64("height", height),
					slog.Any("error", err))
				return err
			}
			return nil
		}()

		if err != nil {
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

		hashes := make([][]byte, 0, len(internalTx.CallTrace.Result))
		for _, trace := range internalTx.CallTrace.Result {
			hashBytes, err := util.HexToBytes(trace.TxHash)
			if err != nil {
				return fmt.Errorf("failed to convert tx hash %s: %w", trace.TxHash, err)
			}
			hashes = append(hashes, hashBytes)
		}

		hashIdMap, err := util.GetOrCreateEvmTxHashIds(tx, hashes, true)
		if err != nil {
			return fmt.Errorf("failed to create hash dictionary entries: %w", err)
		}

		var (
			allInternalTxs []types.CollectedEvmInternalTx
			allEdges       []types.CollectedEvmInternalTxAccount
		)
		for _, trace := range internalTx.CallTrace.Result {
			if trace.Error != "" {
				i.logger.Info("skip indexing in failed transaction",
					slog.Int64("height", internalTx.Height),
					slog.String("tx_hash", trace.TxHash),
					slog.String("error", trace.Error))
				continue
			}
			height := internalTx.Height
			hashHex := strings.ToLower(strings.TrimPrefix(trace.TxHash, "0x"))
			hashId, ok := hashIdMap[hashHex]
			if !ok {
				return types.NewNotFoundError(fmt.Sprintf("hash ID for hash %s", hashHex))
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
				Calls:   trace.Result.Calls,
			}

			txInfo := &InternalTxInfo{
				Height:      height,
				HashId:      hashId,
				Index:       0,  // Top-level starts at index 0
				ParentIndex: -1, // Top-level has no parent
			}

			txResults, err := processInternalCall(tx, txInfo, &topLevelCall, &seqInfo, &allEdges)
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

		if len(allEdges) > 0 {
			if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(allEdges, batchSize).Error; err != nil {
				i.logger.Error("failed to create internal tx account edges", slog.Int64("height", internalTx.Height), slog.Any("error", err))
				return err
			}
		}

		// Update the sequence info
		if err := tx.Clauses(orm.UpdateAllWhenConflict).Create(&seqInfo).Error; err != nil {
			return err
		}

		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
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
