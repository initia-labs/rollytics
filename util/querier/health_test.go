package querier

import (
	"sync"
	"testing"
	"time"
)

func resetHealthTracker() {
	healthTrackerMu.Lock()
	healthTracker = make(map[string]*endpointHealth)
	healthTrackerMu.Unlock()
}

func TestGetEndpointHealth(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://test-endpoint.com"

	// First call should create new health tracker
	h1 := getEndpointHealth(endpoint)
	if h1 == nil {
		t.Fatal("Expected non-nil health tracker")
	}

	// Second call should return the same instance
	h2 := getEndpointHealth(endpoint)
	if h1 != h2 {
		t.Error("Expected same health tracker instance")
	}

	// Different endpoint should get different instance
	h3 := getEndpointHealth("https://different-endpoint.com")
	if h1 == h3 {
		t.Error("Expected different health tracker for different endpoint")
	}
}

func TestRecordEndpointSuccess(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://test-success.com"

	// Record some failures first
	recordEndpointFailure(endpoint)
	recordEndpointFailure(endpoint)
	recordEndpointFailure(endpoint)

	h := getEndpointHealth(endpoint)
	if h.consecutiveFailures.Load() != 3 {
		t.Errorf("Expected 3 failures, got %d", h.consecutiveFailures.Load())
	}

	// Record success should reset failures to 0
	recordEndpointSuccess(endpoint)
	if h.consecutiveFailures.Load() != 0 {
		t.Errorf("Expected 0 failures after success, got %d", h.consecutiveFailures.Load())
	}
}

func TestRecordEndpointFailure(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://test-failure.com"

	h := getEndpointHealth(endpoint)
	initialFailures := h.consecutiveFailures.Load()

	// Record a failure
	recordEndpointFailure(endpoint)
	if h.consecutiveFailures.Load() != initialFailures+1 {
		t.Errorf("Expected failures to increment by 1")
	}

	// Check that last failure time was set
	lastFailure := h.lastFailureTime.Load()
	if lastFailure == 0 {
		t.Error("Expected last failure time to be set")
	}

	// Record another failure
	time.Sleep(1 * time.Millisecond)
	recordEndpointFailure(endpoint)
	if h.consecutiveFailures.Load() != initialFailures+2 {
		t.Errorf("Expected failures to increment by 2")
	}

	// Last failure time should be updated
	newLastFailure := h.lastFailureTime.Load()
	if newLastFailure <= lastFailure {
		t.Error("Expected last failure time to be updated")
	}
}

func TestIsEndpointHealthyThresholds(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://test-threshold.com"

	// New endpoint should be healthy
	if !isEndpointHealthy(endpoint) {
		t.Error("New endpoint should be healthy")
	}

	// Endpoint with 1 failure should be healthy
	recordEndpointFailure(endpoint)
	if !isEndpointHealthy(endpoint) {
		t.Error("Endpoint with 1 failure should be healthy")
	}

	// Endpoint with 2 failures should be healthy
	recordEndpointFailure(endpoint)
	if !isEndpointHealthy(endpoint) {
		t.Error("Endpoint with 2 failures should be healthy")
	}

	// Endpoint with 3 failures should be unhealthy (at threshold)
	recordEndpointFailure(endpoint)
	if isEndpointHealthy(endpoint) {
		t.Error("Endpoint with 3 failures should be unhealthy")
	}

	// Endpoint with 4 failures should be unhealthy
	recordEndpointFailure(endpoint)
	if isEndpointHealthy(endpoint) {
		t.Error("Endpoint with 4 failures should be unhealthy")
	}
}

func TestIsEndpointHealthyRecovery(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://test-recovery.com"

	// Make endpoint unhealthy
	recordEndpointFailure(endpoint)
	recordEndpointFailure(endpoint)
	recordEndpointFailure(endpoint)

	if isEndpointHealthy(endpoint) {
		t.Error("Endpoint should be unhealthy after 3 failures")
	}

	// Manually set last failure time to past (simulate recovery timeout)
	h := getEndpointHealth(endpoint)
	pastTime := time.Now().Add(-recoveryTimeout - 1*time.Second).UnixNano()
	h.lastFailureTime.Store(pastTime)

	// Should be healthy again after recovery timeout
	if !isEndpointHealthy(endpoint) {
		t.Error("Endpoint should be healthy after recovery timeout")
	}
}

func TestFindHealthyEndpoint(t *testing.T) {
	resetHealthTracker()

	endpoints := []string{
		"https://endpoint1-find.com",
		"https://endpoint2-find.com",
		"https://endpoint3-find.com",
	}

	// All healthy - should return 0
	idx := findHealthyEndpoint(endpoints)
	if idx != 0 {
		t.Errorf("Expected index 0 when all healthy, got %d", idx)
	}

	// Make first endpoint unhealthy
	recordEndpointFailure(endpoints[0])
	recordEndpointFailure(endpoints[0])
	recordEndpointFailure(endpoints[0])

	// Should return 1 (second endpoint)
	idx = findHealthyEndpoint(endpoints)
	if idx != 1 {
		t.Errorf("Expected index 1, got %d", idx)
	}

	// Make second endpoint unhealthy too
	recordEndpointFailure(endpoints[1])
	recordEndpointFailure(endpoints[1])
	recordEndpointFailure(endpoints[1])

	// Should return 2 (third endpoint)
	idx = findHealthyEndpoint(endpoints)
	if idx != 2 {
		t.Errorf("Expected index 2, got %d", idx)
	}

	// Make all endpoints unhealthy
	recordEndpointFailure(endpoints[2])
	recordEndpointFailure(endpoints[2])
	recordEndpointFailure(endpoints[2])

	// Should return 0 (fallback to first when all unhealthy)
	idx = findHealthyEndpoint(endpoints)
	if idx != 0 {
		t.Errorf("Expected index 0 when all unhealthy, got %d", idx)
	}
}

func TestConcurrentHealthOperations(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://concurrent-test.com"
	concurrency := 100
	iterations := 50

	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Simulate concurrent failures and successes
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if workerID%2 == 0 {
					recordEndpointFailure(endpoint)
				} else {
					recordEndpointSuccess(endpoint)
				}
				// Also check health concurrently
				_ = isEndpointHealthy(endpoint)
			}
		}(i)
	}

	wg.Wait()

	// Verify no race conditions occurred (test should not panic)
	h := getEndpointHealth(endpoint)
	failures := h.consecutiveFailures.Load()

	// Failures should be either 0 (from success) or some positive number (from failures)
	// The exact value is non-deterministic due to concurrent operations
	if failures < 0 {
		t.Errorf("Failures should not be negative, got %d", failures)
	}
}

func TestConcurrentGetEndpointHealth(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://concurrent-get-test.com"
	concurrency := 100

	var wg sync.WaitGroup
	wg.Add(concurrency)

	healthInstances := make([]*endpointHealth, concurrency)

	// Simulate concurrent first-time access (double-checked locking test)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			healthInstances[idx] = getEndpointHealth(endpoint)
		}(i)
	}

	wg.Wait()

	// All goroutines should get the same instance
	firstInstance := healthInstances[0]
	for i := 1; i < concurrency; i++ {
		if healthInstances[i] != firstInstance {
			t.Error("Expected all concurrent getEndpointHealth calls to return the same instance")
			break
		}
	}

	// Verify only one entry was created
	healthTrackerMu.RLock()
	count := 0
	for range healthTracker {
		count++
	}
	healthTrackerMu.RUnlock()

	if count != 1 {
		t.Errorf("Expected health tracker map to have 1 entry, got %d", count)
	}
}

func TestHealthTrackingIndependentEndpoints(t *testing.T) {
	resetHealthTracker()

	endpoints := []string{
		"https://independent1.com",
		"https://independent2.com",
		"https://independent3.com",
	}

	// Record different failure counts for each endpoint
	recordEndpointFailure(endpoints[0])

	recordEndpointFailure(endpoints[1])
	recordEndpointFailure(endpoints[1])

	recordEndpointFailure(endpoints[2])
	recordEndpointFailure(endpoints[2])
	recordEndpointFailure(endpoints[2])

	// Verify each endpoint has correct failure count
	h0 := getEndpointHealth(endpoints[0])
	if h0.consecutiveFailures.Load() != 1 {
		t.Errorf("Endpoint 0 expected 1 failure, got %d", h0.consecutiveFailures.Load())
	}

	h1 := getEndpointHealth(endpoints[1])
	if h1.consecutiveFailures.Load() != 2 {
		t.Errorf("Endpoint 1 expected 2 failures, got %d", h1.consecutiveFailures.Load())
	}

	h2 := getEndpointHealth(endpoints[2])
	if h2.consecutiveFailures.Load() != 3 {
		t.Errorf("Endpoint 2 expected 3 failures, got %d", h2.consecutiveFailures.Load())
	}

	// Verify health status
	if !isEndpointHealthy(endpoints[0]) {
		t.Error("Endpoint 0 should be healthy with 1 failure")
	}
	if !isEndpointHealthy(endpoints[1]) {
		t.Error("Endpoint 1 should be healthy with 2 failures")
	}
	if isEndpointHealthy(endpoints[2]) {
		t.Error("Endpoint 2 should be unhealthy with 3 failures")
	}
}

func TestSuccessResetsFailureCount(t *testing.T) {
	resetHealthTracker()

	endpoint := "https://reset-test.com"

	// Record 2 failures
	recordEndpointFailure(endpoint)
	recordEndpointFailure(endpoint)

	h := getEndpointHealth(endpoint)
	if h.consecutiveFailures.Load() != 2 {
		t.Errorf("Expected 2 failures, got %d", h.consecutiveFailures.Load())
	}

	// Record success - should reset to 0
	recordEndpointSuccess(endpoint)

	if h.consecutiveFailures.Load() != 0 {
		t.Errorf("Expected 0 failures after success, got %d", h.consecutiveFailures.Load())
	}

	// Record 2 more failures
	recordEndpointFailure(endpoint)
	recordEndpointFailure(endpoint)

	// Should have 2 failures again (not 4)
	if h.consecutiveFailures.Load() != 2 {
		t.Errorf("Expected 2 failures after reset and new failures, got %d", h.consecutiveFailures.Load())
	}

	// Should still be healthy (threshold is 3)
	if !isEndpointHealthy(endpoint) {
		t.Error("Should be healthy with 2 failures after reset")
	}
}
