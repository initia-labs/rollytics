package internaltx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	exttypes "github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/sentry_integration"
	"github.com/initia-labs/rollytics/types"
)

const ExtensionName = "internal-tx"

var _ exttypes.Extension = (*InternalTxExtension)(nil)

// WorkItem represents a work item containing scraped internal transaction data
type WorkItem struct {
	Height    int64
	CallTrace *DebugCallTraceBlockResponse
}

// WorkQueue represents a thread-safe queue for work items using channels
type WorkQueue struct {
	ch      chan *WorkItem
	maxSize int
}

// NewWorkQueue creates a new work queue with the specified maximum size
func NewWorkQueue(maxSize int) *WorkQueue {
	return &WorkQueue{
		ch:      make(chan *WorkItem, maxSize),
		maxSize: maxSize,
	}
}

// Push adds a work item to the queue, blocking if the queue is full
func (wq *WorkQueue) Push(ctx context.Context, item *WorkItem) error {
	select {
	case wq.ch <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Pop removes and returns a work item from the queue, blocking if the queue is empty
func (wq *WorkQueue) Pop(ctx context.Context) (*WorkItem, error) {
	select {
	case item := <-wq.ch:
		return item, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Size returns the current size of the queue
func (wq *WorkQueue) Size() int {
	return len(wq.ch)
}

// IsNotFull returns true if the queue is not at maximum capacity
func (wq *WorkQueue) IsNotFull() bool {
	return len(wq.ch) < wq.maxSize
}

// IsNotEmpty returns true if the queue has items
func (wq *WorkQueue) IsNotEmpty() bool {
	return len(wq.ch) > 0
}

// Close closes the underlying channel (call this when shutting down)
func (wq *WorkQueue) Close() {
	close(wq.ch)
}

// InternalTxExtension is responsible for collecting and indexing internal transactions.
type InternalTxExtension struct {
	cfg                *config.Config
	logger             *slog.Logger
	db                 *orm.Database
	lastProducedHeight int64 // Last produced/queued height
	workQueue          *WorkQueue
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *InternalTxExtension {
	if cfg.GetVmType() != types.EVM || !cfg.InternalTxEnabled() {
		return nil
	}

	return &InternalTxExtension{
		cfg:       cfg,
		logger:    logger.With("extension", ExtensionName),
		db:        db,
		workQueue: NewWorkQueue(cfg.GetInternalTxConfig().GetQueueSize()),
	}
}

func (i *InternalTxExtension) Initialize(ctx context.Context) error {
	// Initialize last height with context
	var lastItx types.CollectedEvmInternalTx
	if err := i.db.WithContext(ctx).
		Model(types.CollectedEvmInternalTx{}).Order("height desc").First(&lastItx).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		i.logger.Error("failed to get the last block height", slog.Any("error", err))
		return err
	}

	i.lastProducedHeight = lastItx.Height
	return nil
}

func (i *InternalTxExtension) Run(ctx context.Context) error {
	if err := CheckNodeVersion(i.cfg); err != nil {
		i.logger.Warn("skipping internal transaction indexing", slog.Any("reason", err.Error()))
		return nil
	}
	err := i.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize internal transaction extension: %w", err)
	}

	// Ensure work queue is closed when function exits
	defer i.workQueue.Close()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return i.runProducer(ctx)
	})

	g.Go(func() error {
		return i.runConsumer(ctx)
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	i.logger.Info("internal tx extension shut down gracefully")
	return nil
}

// runProducer finds new block heights, scrapes data, and adds work items to the queue
func (i *InternalTxExtension) runProducer(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			i.logger.Info("producer shutting down")
			return ctx.Err()
		default:
			// Only produce work if queue is not full
			if i.workQueue.IsNotFull() {
				// Use batch processing instead of single height processing
				if err := i.produceBatchWork(ctx); err != nil {
					if errors.Is(err, context.Canceled) {
						return err
					}
					i.logger.Error("failed to produce batch work",
						slog.Any("error", err),
						slog.Int64("last_height", i.lastProducedHeight))

					time.Sleep(i.cfg.GetInternalTxConfig().GetPollInterval())
				}
			} else {
				// Queue is full, wait a bit before checking again
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// runConsumer processes work items from the queue
func (i *InternalTxExtension) runConsumer(ctx context.Context) error {
	i.logger.Info("consumer started")

	for {
		workItem, err := i.workQueue.Pop(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				i.logger.Info("consumer shutting down")
				return err
			}
			i.logger.Error("failed to pop work item from queue", slog.Any("error", err))
			continue
		}

		if err := i.consumeWork(ctx, workItem); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			i.logger.Error("failed to consume work item",
				slog.Int64("height", workItem.Height),
				slog.Any("error", err))
			// Continue processing other work items even if one fails
		} else {
			i.logger.Debug("successfully processed work item", slog.Int64("height", workItem.Height))
		}
	}
}

// Combined function that gets both batch size and heights in one query
func (i *InternalTxExtension) getBatchHeights(ctx context.Context) ([]int64, error) {
	var heights []int64

	// Check context before DB operation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	availableSpace := i.workQueue.maxSize - i.workQueue.Size()
	if availableSpace <= 0 {
		return heights, nil // No space in queue
	}
	batchSize := min(i.cfg.GetInternalTxConfig().GetBatchSize(), availableSpace)

	// Single query to get heights - the database will naturally limit to available blocks
	if err := i.db.WithContext(ctx).
		Model(&types.CollectedBlock{}).
		Where("chain_id = ?", i.cfg.GetChainId()).
		Where("height > ?", i.lastProducedHeight).
		Where("tx_count > 0").
		Order("height ASC").
		Limit(batchSize).
		Pluck("height", &heights).Error; err != nil {
		return nil, fmt.Errorf("failed to query blocks: %w", err)
	}

	return heights, nil
}

// produceBatchWork scrapes multiple heights concurrently in batches
func (i *InternalTxExtension) produceBatchWork(ctx context.Context) error {
	transaction, ctx := sentry_integration.StartSentryTransaction(ctx, "(internal-tx) produceBatchWork", "Producing batch work items")
	defer transaction.Finish()

	heights, err := i.getBatchHeights(ctx)
	if err != nil {
		return fmt.Errorf("failed to get batch heights: %w", err)
	}

	// If no heights available, return early
	if len(heights) == 0 {
		return nil
	}

	workItems, err := i.scrapeBatch(ctx, heights)
	if err != nil {
		return fmt.Errorf("failed to scrape batch starting at height %d: %w", heights[0], err)
	}

	// Push all work items to queue
	for _, workItem := range workItems {
		if err := i.workQueue.Push(ctx, workItem); err != nil {
			return fmt.Errorf("failed to add work item to queue: %w", err)
		}
	}

	i.lastProducedHeight = heights[len(heights)-1]
	return nil
}

// scrapeBatch scrapes multiple heights concurrently and waits for all to complete
func (i *InternalTxExtension) scrapeBatch(ctx context.Context, heights []int64) ([]*WorkItem, error) {
	g, ctx := errgroup.WithContext(ctx)

	workItems := make([]*WorkItem, len(heights))

	for idx, height := range heights {
		idx, height := idx, height // Capture loop variables

		g.Go(func() error {
			return func(idx int, height int64) error {
				workItem, err := i.scrapeHeight(ctx, height)
				if err != nil {
					return fmt.Errorf("failed to scrape height %d: %w", height, err)
				}
				workItems[idx] = workItem
				return nil
			}(idx, height)
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := make([]*WorkItem, 0, len(workItems))
	for _, item := range workItems {
		if item != nil {
			result = append(result, item)
		}
	}

	return result, nil
}

// scrapeHeight scrapes internal transaction data for a single height
func (i *InternalTxExtension) scrapeHeight(ctx context.Context, height int64) (*WorkItem, error) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	// Scrape internal transaction data
	callTraceRes, err := TraceCallByBlock(ctx, i.cfg, client, height)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			i.logger.Info("scraping cancelled", slog.Int64("height", height))
			return nil, err
		}
		i.logger.Error("failed to scrape internal tx",
			slog.Int64("height", height),
			slog.Any("error", err))
		return nil, err
	}

	i.logger.Info("scraped internal txs", slog.Int64("height", height))

	return &WorkItem{
		Height:    height,
		CallTrace: callTraceRes,
	}, nil
}

// consumeWork processes a work item by saving it to the database
func (i *InternalTxExtension) consumeWork(ctx context.Context, workItem *WorkItem) error {
	transaction, ctx := sentry_integration.StartSentryTransaction(ctx, "(internal-tx) consumeWork", "Consuming work item for height "+strconv.FormatInt(workItem.Height, 10))
	defer transaction.Finish()

	// Convert WorkItem to InternalTxResult for compatibility with existing method
	internalTxResult := &InternalTxResult{
		Height:    workItem.Height,
		CallTrace: workItem.CallTrace,
	}

	// Use existing CollectInternalTxs method to save to database
	if err := i.CollectInternalTxs(ctx, i.db, internalTxResult); err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		i.logger.Error("failed to collect internal txs",
			slog.Int64("height", workItem.Height),
			slog.Any("error", err))
		return err
	}

	return nil
}

// Name returns the name of the extension
func (i *InternalTxExtension) Name() string {
	return ExtensionName
}
