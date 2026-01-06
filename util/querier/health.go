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

// healthState represents the state of an endpoint
type healthState struct {
	failures      int32
	lastFailureAt time.Time
}

// endpointHealth tracks health status of an endpoint
// Uses atomic.Value to avoid race conditions between failure count and timestamp
type endpointHealth struct {
	state atomic.Value // stores *healthState
}

func (h *endpointHealth) load() *healthState {
	v := h.state.Load()
	if v == nil {
		return &healthState{}
	}
	return v.(*healthState)
}

func (h *endpointHealth) store(s *healthState) {
	h.state.Store(s)
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
	h.store(&healthState{
		failures:      0,
		lastFailureAt: time.Time{},
	})
}

// recordEndpointFailure increments failure count and updates last failure time atomically
func recordEndpointFailure(endpoint string) {
	h := getEndpointHealth(endpoint)
	now := time.Now()

	// Atomically update by creating new state based on old state
	oldState := h.load()
	newState := &healthState{
		failures:      oldState.failures + 1,
		lastFailureAt: now,
	}
	h.store(newState)
}

// isEndpointHealthy checks if an endpoint is healthy enough to use
func isEndpointHealthy(endpoint string) bool {
	h := getEndpointHealth(endpoint)
	state := h.load()

	// If below failure threshold, endpoint is healthy
	if state.failures < failureThreshold {
		return true
	}

	// At or above threshold - check if recovery timeout has passed
	if state.lastFailureAt.IsZero() {
		// Should not happen, but treat as unhealthy to be safe
		return false
	}

	timeSinceFailure := time.Since(state.lastFailureAt)
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
