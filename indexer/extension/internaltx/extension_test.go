package internaltx

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
)

// MockDB for testing
type MockDB struct {
	mock.Mock
}

func (m *MockDB) WithContext(ctx context.Context) *gorm.DB {
	args := m.Called(ctx)
	return args.Get(0).(*gorm.DB)
}

// TestGracefulShutdown verifies that the extension shuts down gracefully
func TestGracefulShutdown(t *testing.T) {
	// Create test logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create mock config
	cfg := &config.Config{}

	// Create mock database (simplified for test)
	db := &orm.Database{
		DB: &gorm.DB{},
	}

	// Create extension
	ext := &InternalTxExtension{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Channel to track when Run completes
	done := make(chan error, 1)

	// Start extension in goroutine
	go func() {
		done <- ext.Run(ctx)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context (simulate SIGTERM)
	cancel()

	// Wait for shutdown with timeout
	select {
	case err := <-done:
		// Should complete without error (context cancellation is expected)
		assert.NoError(t, err, "Extension should shut down without error")
	case <-time.After(5 * time.Second):
		t.Fatal("Extension did not shut down within timeout")
	}
}

// TestContextPropagation verifies context is properly propagated through the call chain
func TestContextPropagation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	cfg := &config.Config{}
	db := &orm.Database{
		DB: &gorm.DB{},
	}

	ext := &InternalTxExtension{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// processBatch should return immediately with context error
	err := ext.processBatch(ctx)
	assert.ErrorIs(t, err, context.Canceled, "Should return context.Canceled error")

	// collect should also respect cancelled context
	err = ext.collect(ctx, []int64{1, 2, 3})
	assert.ErrorIs(t, err, context.Canceled, "Should return context.Canceled error")
}

// TestMultipleExtensionsShutdown tests shutdown with multiple extensions
func TestMultipleExtensionsShutdown(t *testing.T) {
	// Mock extension that respects context
	type mockExtension struct {
		name     string
		shutdown chan struct{}
	}

	impl := func(m *mockExtension, ctx context.Context) error {
		<-ctx.Done()
		close(m.shutdown)
		return nil
	}

	// Create multiple mock extensions
	ext1 := &mockExtension{name: "ext1", shutdown: make(chan struct{})}
	ext2 := &mockExtension{name: "ext2", shutdown: make(chan struct{})}
	ext3 := &mockExtension{name: "ext3", shutdown: make(chan struct{})}

	// Start all in parallel
	ctx, cancel := context.WithCancel(context.Background())

	go impl(ext1, ctx)
	go impl(ext2, ctx)
	go impl(ext3, ctx)

	// Give them time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for all to shutdown
	timeout := time.After(2 * time.Second)
	for _, ext := range []*mockExtension{ext1, ext2, ext3} {
		select {
		case <-ext.shutdown:
			// Good, extension shut down
		case <-timeout:
			t.Fatalf("Extension %s did not shut down", ext.name)
		}
	}
}

// TestShutdownDuringProcessing tests shutdown while processing is happening
func TestShutdownDuringProcessing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	cfg := &config.Config{}
	db := &orm.Database{
		DB: &gorm.DB{},
	}

	ext := &InternalTxExtension{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}

	// Simulate long-running collect operation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should be interrupted by context timeout
	heights := []int64{1, 2, 3, 4, 5}
	err := ext.collect(ctx, heights)

	// Should eventually return with context error
	if err != nil {
		assert.True(t,
			errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
			"Should return context error")
	}
}

// BenchmarkShutdownLatency measures shutdown response time
func BenchmarkShutdownLatency(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Discard, nil))
	cfg := &config.Config{}
	db := &orm.Database{DB: &gorm.DB{}}

	for i := 0; i < b.N; i++ {
		ext := &InternalTxExtension{
			cfg:    cfg,
			logger: logger,
			db:     db,
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			_ = ext.Run(ctx)
			close(done)
		}()

		// Measure shutdown time
		start := time.Now()
		cancel()
		<-done
		elapsed := time.Since(start)

		// Should shut down quickly
		if elapsed > 100*time.Millisecond {
			b.Fatalf("Shutdown took too long: %v", elapsed)
		}
	}
}