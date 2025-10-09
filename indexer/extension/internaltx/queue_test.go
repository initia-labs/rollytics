package internaltx

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkQueue(t *testing.T) {
	wq := NewWorkQueue(10)
	assert.NotNil(t, wq)
	assert.Equal(t, 10, wq.maxSize)
	assert.Equal(t, 0, wq.Size())
	assert.True(t, wq.IsNotFull())
	assert.False(t, wq.IsNotEmpty())
}

func TestWorkQueue_PushPop(t *testing.T) {
	wq := NewWorkQueue(5)
	ctx := context.Background()

	workItem := &WorkItem{Height: 100}

	// Test push
	err := wq.Push(ctx, workItem)
	require.NoError(t, err)
	assert.Equal(t, 1, wq.Size())
	assert.True(t, wq.IsNotEmpty())

	// Test pop
	poppedItem, err := wq.Pop(ctx)
	require.NoError(t, err)
	assert.Equal(t, workItem, poppedItem)
	assert.Equal(t, 0, wq.Size())
	assert.False(t, wq.IsNotEmpty())
}

func TestWorkQueue_ThreadSafety(t *testing.T) {
	wq := NewWorkQueue(100)
	ctx := context.Background()

	const numWorkers = 10
	const itemsPerWorker = 50
	const totalItems = numWorkers * itemsPerWorker

	var wg sync.WaitGroup
	itemsProduced := make(chan int64, totalItems)
	itemsConsumed := make(chan int64, totalItems)

	// Start producers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < itemsPerWorker; j++ {
				height := int64(workerID*1000 + j)
				item := &WorkItem{Height: height}
				err := wq.Push(ctx, item)
				require.NoError(t, err)
				itemsProduced <- height
			}
		}(i)
	}

	// Start consumers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < itemsPerWorker; j++ {
				item, err := wq.Pop(ctx)
				require.NoError(t, err)
				itemsConsumed <- item.Height
			}
		}()
	}

	wg.Wait()
	close(itemsProduced)
	close(itemsConsumed)

	// Verify all items were processed
	produced := make(map[int64]bool)
	for height := range itemsProduced {
		produced[height] = true
	}

	consumed := make(map[int64]bool)
	for height := range itemsConsumed {
		consumed[height] = true
	}

	assert.Equal(t, totalItems, len(produced))
	assert.Equal(t, totalItems, len(consumed))
	assert.Equal(t, produced, consumed)
	assert.Equal(t, 0, wq.Size())
}

func TestWorkQueue_StateCheckers(t *testing.T) {
	wq := NewWorkQueue(3)
	ctx := context.Background()

	// Initially empty
	assert.Equal(t, 0, wq.Size())
	assert.True(t, wq.IsNotFull())
	assert.False(t, wq.IsNotEmpty())

	// Add items
	for i := 0; i < 3; i++ {
		err := wq.Push(ctx, &WorkItem{Height: int64(i)})
		require.NoError(t, err)
		assert.Equal(t, i+1, wq.Size())
		assert.True(t, wq.IsNotEmpty())
	}

	// Queue is full
	assert.False(t, wq.IsNotFull())

	// Remove all items
	for i := 0; i < 3; i++ {
		_, err := wq.Pop(ctx)
		require.NoError(t, err)
	}

	// Back to empty
	assert.Equal(t, 0, wq.Size())
	assert.True(t, wq.IsNotFull())
	assert.False(t, wq.IsNotEmpty())
}

func TestWorkQueue_Close(t *testing.T) {
	wq := NewWorkQueue(5)
	ctx := context.Background()

	// Add some items
	for i := 0; i < 3; i++ {
		err := wq.Push(ctx, &WorkItem{Height: int64(i)})
		require.NoError(t, err)
	}

	// Close queue
	wq.Close()

	// Can still pop existing items
	for i := 0; i < 3; i++ {
		item, err := wq.Pop(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(i), item.Height)
	}

	// Pop from closed empty queue returns nil
	item, err := wq.Pop(ctx)
	assert.NoError(t, err)
	assert.Nil(t, item)
}

func TestWorkQueue_ConsumeOrderCorrect(t *testing.T) {
	wq := NewWorkQueue(10)
	ctx := context.Background()

	// Test FIFO ordering with sequential heights
	heights := []int64{100, 101, 102, 103, 104, 105}

	// Push items in order
	for _, height := range heights {
		workItem := &WorkItem{Height: height}
		err := wq.Push(ctx, workItem)
		require.NoError(t, err)
	}

	// Pop items and verify they come out in the same order (FIFO)
	for i, expectedHeight := range heights {
		item, err := wq.Pop(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedHeight, item.Height,
			"Item at position %d should have height %d, got %d", i, expectedHeight, item.Height)
	}

	// Queue should be empty now
	assert.Equal(t, 0, wq.Size())
	assert.False(t, wq.IsNotEmpty())
}

func TestWorkQueue_SingleWorkerPopWaitsForSequentialPush(t *testing.T) {
	wq := NewWorkQueue(15) // Make queue larger than test size to avoid blocking
	ctx := context.Background()

	const numTasks = 10

	// Channel to coordinate test phases
	workerReady := make(chan struct{})
	allTasksPushed := make(chan struct{})

	// Track what was popped by the single worker
	poppedItems := make([]*WorkItem, 0, numTasks)

	// Start single worker that waits for data
	go func() {
		// Signal that worker is ready
		close(workerReady)

		// Single worker pops items as they become available
		for i := 0; i < numTasks; i++ {
			// This will block until an item is available
			item, err := wq.Pop(ctx)
			require.NoError(t, err)
			require.NotNil(t, item)

			poppedItems = append(poppedItems, item)

			// Log progress (optional)
			t.Logf("Worker popped item %d with height %d", i+1, item.Height)
		}

		close(allTasksPushed)
	}()

	// Wait for worker to be ready
	<-workerReady

	// Give worker a moment to start waiting
	time.Sleep(50 * time.Millisecond)

	// Push 10 tasks one at a time
	pushedItems := make([]*WorkItem, 0, numTasks)
	for i := 0; i < numTasks; i++ {
		workItem := &WorkItem{Height: int64(100 + i)}
		pushedItems = append(pushedItems, workItem)

		t.Logf("Pushing task %d with height %d", i+1, workItem.Height)
		err := wq.Push(ctx, workItem)
		require.NoError(t, err)

		// Small delay between pushes to simulate real-world timing
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for worker to process all items
	<-allTasksPushed

	// Verify results
	assert.Equal(t, numTasks, len(poppedItems))
	assert.Equal(t, 0, wq.Size()) // Queue should be empty after worker processed all items

	// Verify FIFO order - items should be popped in the same order they were pushed
	for i, poppedItem := range poppedItems {
		expectedHeight := int64(100 + i)
		assert.Equal(t, expectedHeight, poppedItem.Height,
			"Item %d should have height %d, got %d", i, expectedHeight, poppedItem.Height)
	}

	// Verify that all pushed items were consumed
	assert.Equal(t, len(pushedItems), len(poppedItems))
}
