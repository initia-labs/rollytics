package internaltx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
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

// WorkQueue represents a thread-safe queue for work items
type WorkQueue struct {
	items    []*WorkItem
	maxSize  int
	mu       sync.RWMutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
}

// NewWorkQueue creates a new work queue with the specified maximum size
func NewWorkQueue(maxSize int) *WorkQueue {
	wq := &WorkQueue{
		items:   make([]*WorkItem, 0),
		maxSize: maxSize,
	}
	wq.notEmpty = sync.NewCond(&wq.mu)
	wq.notFull = sync.NewCond(&wq.mu)
	return wq
}

// Push adds a work item to the queue, blocking if the queue is full
func (wq *WorkQueue) Push(ctx context.Context, item *WorkItem) error {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	for len(wq.items) >= wq.maxSize {
		wq.mu.Unlock()
		select {
		case <-ctx.Done():
			wq.mu.Lock() // Re-acquire for defer
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Brief wait before retrying
		}
		wq.mu.Lock()
	}

	wq.items = append(wq.items, item)
	wq.notEmpty.Signal()
	return nil
}

// Pop removes and returns a work item from the queue, blocking if the queue is empty
func (wq *WorkQueue) Pop(ctx context.Context) (*WorkItem, error) {
	wq.mu.Lock()
	defer wq.mu.Unlock()

	for len(wq.items) == 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		wq.notEmpty.Wait()
	}

	item := wq.items[0]
	wq.items = wq.items[1:]
	wq.notFull.Signal()
	return item, nil
}

// Size returns the current size of the queue
func (wq *WorkQueue) Size() int {
	wq.mu.RLock()
	defer wq.mu.RUnlock()
	return len(wq.items)
}

// IsNotFull returns true if the queue is not at maximum capacity
func (wq *WorkQueue) IsNotFull() bool {
	wq.mu.RLock()
	defer wq.mu.RUnlock()
	return len(wq.items) < wq.maxSize
}

// IsNotEmpty returns true if the queue has items
func (wq *WorkQueue) IsNotEmpty() bool {
	wq.mu.RLock()
	defer wq.mu.RUnlock()
	return len(wq.items) > 0
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

	// Check if the next height (lastItx.Height + 1) exists and has transactions
	nextHeight := lastItx.Height + 1
	var exists bool
	if err := i.db.WithContext(ctx).
		Model(&types.CollectedBlock{}).
		Where("chain_id = ?", i.cfg.GetChainId()).
		Where("height = ?", nextHeight).
		Where("tx_count > 0").
		Select("count(*) > 0").
		Find(&exists).Error; err != nil {
		return fmt.Errorf("failed to check if next block exists: %w", err)
	}

	if exists {
		i.lastProducedHeight = nextHeight
	} else {
		// No next block available yet, stay at current height
		i.lastProducedHeight = lastItx.Height
	}

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

	go i.runProducer(ctx)
	go i.runConsumer(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	i.logger.Info("internal tx extension shutting down gracefully",
		slog.String("reason", ctx.Err().Error()))
	return ctx.Err()
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
				if err := i.produceWork(ctx); err != nil {
					if errors.Is(err, context.Canceled) {
						return err
					}
					i.logger.Error("failed to produce work",
						slog.Any("error", err),
						slog.Int64("last_height", i.lastProducedHeight))

					// TODO: revisit wait logic
					if i.lastProducedHeight%int64(i.cfg.GetInternalTxConfig().GetBatchSize()) == 0 {
						time.Sleep(i.cfg.GetInternalTxConfig().GetPollInterval())
					}
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

// produceWork finds new block heights, scrapes internal transaction data, and queues work items
func (i *InternalTxExtension) produceWork(ctx context.Context) error {
	transaction, ctx := sentry_integration.StartSentryTransaction(ctx, "produceWork", "Finding and scraping new heights")
	defer transaction.Finish()

	// Check context before DB operation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Scrape internal transaction data
	workItem, err := i.scrapeHeight(ctx, i.lastProducedHeight)
	if err != nil {
		return fmt.Errorf("failed to scrape height %d: %w", i.lastProducedHeight, err)
	}

	// Add work item to queue
	if err := i.workQueue.Push(ctx, workItem); err != nil {
		return fmt.Errorf("failed to add work item to queue: %w", err)
	}

	i.logger.Debug("produced work item",
		slog.Int64("height", i.lastProducedHeight),
		slog.Int("queue_size", i.workQueue.Size()))

	// Update lastProducedHeight after successfully queuing
	i.lastProducedHeight += 1
	return nil
}

// scrapeHeight scrapes internal transaction data for a single height
func (i *InternalTxExtension) scrapeHeight(ctx context.Context, height int64) (*WorkItem, error) {
	span, _ := sentry_integration.StartSentrySpan(ctx, "scrapeHeight", "Scraping internal transactions for height "+strconv.FormatInt(height, 10))
	defer span.Finish()

	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	// Scrape internal transaction data
	callTraceRes, err := TraceCallByBlock(i.cfg, client, height)
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
	span, _ := sentry_integration.StartSentrySpan(ctx, "consumeWork", "Processing internal transactions for height "+strconv.FormatInt(workItem.Height, 10))
	defer span.Finish()

	// Convert WorkItem to InternalTxResult for compatibility with existing method
	internalTxResult := &InternalTxResult{
		Height:    workItem.Height,
		CallTrace: workItem.CallTrace,
	}

	// Use existing CollectInternalTxs method to save to database
	if err := i.CollectInternalTxs(i.db, internalTxResult); err != nil {
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
