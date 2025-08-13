package metrics

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// MetricsBatcher provides efficient batching for high-frequency metrics
type MetricsBatcher struct {
	mu            sync.RWMutex
	buffer        *MetricsBuffer
	flushInterval time.Duration
	maxBufferSize int
	done          chan struct{}
	started       atomic.Bool
	flushing      atomic.Bool
}

// MetricsBuffer holds batched metrics before flushing
type MetricsBuffer struct {
	// Concurrent requests delta (can be positive or negative)
	concurrentRequestsDelta int64

	// API latencies grouped by endpoint
	apiLatencies map[string][]float64

	// Semaphore wait times
	semaphoreWaits []float64

	// API requests grouped by endpoint and status
	apiRequests map[string]map[string]int64

	// Rate limit hits grouped by endpoint
	rateLimitHits map[string]int64
}

// MetricsBatcherConfig holds configuration for the batcher
type MetricsBatcherConfig struct {
	FlushInterval time.Duration // How often to flush metrics
	MaxBufferSize int           // Maximum items in buffer before forced flush
}

// DefaultMetricsBatcherConfig returns sensible defaults
func DefaultMetricsBatcherConfig() MetricsBatcherConfig {
	return MetricsBatcherConfig{
		FlushInterval: 5 * time.Second,
		MaxBufferSize: 10000,
	}
}

// NewMetricsBatcher creates a new metrics batcher
func NewMetricsBatcher(config MetricsBatcherConfig) *MetricsBatcher {
	return &MetricsBatcher{
		buffer:        newMetricsBuffer(),
		flushInterval: config.FlushInterval,
		maxBufferSize: config.MaxBufferSize,
		done:          make(chan struct{}),
	}
}

// newMetricsBuffer creates a new empty buffer
func newMetricsBuffer() *MetricsBuffer {
	return &MetricsBuffer{
		apiLatencies:   make(map[string][]float64),
		apiRequests:    make(map[string]map[string]int64),
		rateLimitHits:  make(map[string]int64),
		semaphoreWaits: make([]float64, 0),
	}
}

// Start begins the background flushing routine
func (b *MetricsBatcher) Start() {
	if b.started.Swap(true) {
		return // Already started
	}

	ticker := time.NewTicker(b.flushInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				b.flush()
			case <-b.done:
				b.flush() // Final flush
				return
			}
		}
	}()
}

// Stop stops the background flushing routine
func (b *MetricsBatcher) Stop() {
	if !b.started.Load() {
		return
	}
	b.started.Store(false)
	close(b.done)
}

// RecordConcurrentRequest records a change in concurrent requests
func (b *MetricsBatcher) RecordConcurrentRequest(delta int64) {
	b.mu.Lock()
	b.buffer.concurrentRequestsDelta += delta
	b.checkBufferSize()
	b.mu.Unlock()
}

// RecordAPILatency records API latency for an endpoint
func (b *MetricsBatcher) RecordAPILatency(endpoint string, duration float64) {
	b.mu.Lock()
	b.buffer.apiLatencies[endpoint] = append(b.buffer.apiLatencies[endpoint], duration)
	b.checkBufferSize()
	b.mu.Unlock()
}

// RecordSemaphoreWait records semaphore wait time
func (b *MetricsBatcher) RecordSemaphoreWait(duration float64) {
	b.mu.Lock()
	b.buffer.semaphoreWaits = append(b.buffer.semaphoreWaits, duration)
	b.checkBufferSize()
	b.mu.Unlock()
}

// RecordAPIRequest records an API request with status
func (b *MetricsBatcher) RecordAPIRequest(endpoint, status string) {
	b.mu.Lock()
	if b.buffer.apiRequests[endpoint] == nil {
		b.buffer.apiRequests[endpoint] = make(map[string]int64)
	}
	b.buffer.apiRequests[endpoint][status]++
	b.checkBufferSize()
	b.mu.Unlock()
}

// RecordRateLimitHit records a rate limit hit for an endpoint
func (b *MetricsBatcher) RecordRateLimitHit(endpoint string) {
	b.mu.Lock()
	b.buffer.rateLimitHits[endpoint]++
	b.checkBufferSize()
	b.mu.Unlock()
}

// checkBufferSize checks if buffer needs forced flush (must be called with lock held)
func (b *MetricsBatcher) checkBufferSize() {
	totalItems := len(b.buffer.semaphoreWaits)
	for _, latencies := range b.buffer.apiLatencies {
		totalItems += len(latencies)
	}
	for _, statuses := range b.buffer.apiRequests {
		for range statuses {
			totalItems++
		}
	}
	totalItems += len(b.buffer.rateLimitHits)

	if totalItems >= b.maxBufferSize {
		if b.flushing.CompareAndSwap(false, true) {
			go b.flush() // Async flush to avoid blocking
		}
	}
}

// flush processes all batched metrics and sends them to Prometheus
func (b *MetricsBatcher) flush() {
	defer b.flushing.Store(false)
	b.mu.Lock()
	buffer := b.buffer
	b.buffer = newMetricsBuffer() // Replace with new buffer
	b.mu.Unlock()

	// Process concurrent requests delta
	if buffer.concurrentRequestsDelta != 0 {
		if buffer.concurrentRequestsDelta > 0 {
			for range buffer.concurrentRequestsDelta {
				ConcurrentRequestsActive().Inc()
			}
		} else {
			for range -buffer.concurrentRequestsDelta {
				ConcurrentRequestsActive().Dec()
			}
		}
	}

	// Process API latencies
	for endpoint, latencies := range buffer.apiLatencies {
		for _, latency := range latencies {
			ExternalAPILatency().WithLabelValues(endpoint).Observe(latency)
		}
	}

	// Process semaphore wait times
	for _, waitTime := range buffer.semaphoreWaits {
		SemaphoreWaitDuration().Observe(waitTime)
	}

	// Process API requests
	for endpoint, statuses := range buffer.apiRequests {
		for status, count := range statuses {
			for range count {
				ExternalAPIRequestsTotal().WithLabelValues(endpoint, status).Inc()
			}
		}
	}

	// Process rate limit hits
	for endpoint, count := range buffer.rateLimitHits {
		for range count {
			RateLimitHitsTotal().WithLabelValues(endpoint).Inc()
		}
	}
}

// GetBufferStats returns current buffer statistics for monitoring
func (b *MetricsBatcher) GetBufferStats() map[string]int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := map[string]int{
		"concurrent_requests_delta": int(b.buffer.concurrentRequestsDelta),
		"api_latencies_count":       0,
		"semaphore_waits_count":     len(b.buffer.semaphoreWaits),
		"api_requests_count":        0,
		"rate_limit_hits_count":     len(b.buffer.rateLimitHits),
	}

	for _, latencies := range b.buffer.apiLatencies {
		stats["api_latencies_count"] += len(latencies)
	}

	for _, statuses := range b.buffer.apiRequests {
		for range statuses {
			stats["api_requests_count"]++
		}
	}

	return stats
}

// Global metrics batcher instance
var globalMetricsBatcher *MetricsBatcher
var shutdownOnce sync.Once

func init() {
	// Initialize global metrics batcher with default config
	config := DefaultMetricsBatcherConfig()
	globalMetricsBatcher = NewMetricsBatcher(config)
	globalMetricsBatcher.Start()

	// Set up graceful shutdown
	setupGracefulShutdown()
}

// setupGracefulShutdown sets up signal handlers for graceful shutdown
func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		shutdownGlobalMetricsBatcher()
		signal.Stop(c)
	}()
}

// shutdownGlobalMetricsBatcher performs graceful shutdown of the global metrics batcher
func shutdownGlobalMetricsBatcher() {
	shutdownOnce.Do(func() {
		if globalMetricsBatcher != nil {
			globalMetricsBatcher.Stop()
		}
	})
}

// ShutdownGlobalMetricsBatcher provides a public interface for graceful shutdown
func ShutdownGlobalMetricsBatcher() {
	shutdownGlobalMetricsBatcher()
}

// InitGlobalMetricsBatcher initializes global metrics batcher with custom configuration
func InitGlobalMetricsBatcher(config MetricsBatcherConfig) {
	if globalMetricsBatcher != nil {
		globalMetricsBatcher.Stop()
	}
	globalMetricsBatcher = NewMetricsBatcher(config)
	globalMetricsBatcher.Start()
}

// GetGlobalMetricsBatcherStats returns current buffer statistics for monitoring
func GetGlobalMetricsBatcherStats() map[string]int {
	if globalMetricsBatcher == nil {
		return nil
	}
	return globalMetricsBatcher.GetBufferStats()
}

// GetGlobalMetricsBatcher returns the global metrics batcher instance
func GetGlobalMetricsBatcher() *MetricsBatcher {
	return globalMetricsBatcher
}
