package querier

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Circuit breaker thresholds
	failureThreshold = 3               // Number of consecutive failures before marking unhealthy
	recoveryTimeout  = 5 * time.Minute // Time before retrying an unhealthy endpoint
)

// endpointHealth tracks health status of an endpoint
type endpointHealth struct {
	consecutiveFailures atomic.Int32
	lastFailureTime     atomic.Int64 // Unix nanoseconds
}

// Global health tracker for all endpoints (keyed by endpoint URL)
var (
	healthTrackerMu sync.RWMutex
	healthTracker   = make(map[string]*endpointHealth)
)

// getEndpointHealth returns or creates health status for an endpoint (thread-safe)
func getEndpointHealth(endpoint string) *endpointHealth {
	healthTrackerMu.RLock()
	h, exists := healthTracker[endpoint]
	healthTrackerMu.RUnlock()

	if exists {
		return h
	}

	healthTrackerMu.Lock()
	defer healthTrackerMu.Unlock()

	// Double-check after acquiring write lock
	if h, exists := healthTracker[endpoint]; exists {
		return h
	}

	h = &endpointHealth{}
	healthTracker[endpoint] = h
	return h
}

// recordEndpointSuccess marks an endpoint as healthy
func recordEndpointSuccess(endpoint string) {
	h := getEndpointHealth(endpoint)
	h.consecutiveFailures.Store(0)
}

// recordEndpointFailure increments failure count and updates last failure time
func recordEndpointFailure(endpoint string) {
	h := getEndpointHealth(endpoint)
	h.consecutiveFailures.Add(1)
	h.lastFailureTime.Store(time.Now().UnixNano())
}

// isEndpointHealthy checks if an endpoint is healthy enough to use
func isEndpointHealthy(endpoint string) bool {
	h := getEndpointHealth(endpoint)
	failures := h.consecutiveFailures.Load()

	// If below failure threshold, endpoint is healthy
	if failures < failureThreshold {
		return true
	}

	// If above threshold, check if recovery timeout has passed
	lastFailure := h.lastFailureTime.Load()
	if lastFailure == 0 {
		return true
	}

	timeSinceFailure := time.Since(time.Unix(0, lastFailure))
	return timeSinceFailure >= recoveryTimeout
}

// findHealthyEndpoint returns the index of the first healthy endpoint, or 0 if none are healthy
func findHealthyEndpoint(endpoints []string) int {
	for i, endpoint := range endpoints {
		if isEndpointHealthy(endpoint) {
			return i
		}
	}
	// If no healthy endpoints, start from beginning
	return 0
}
