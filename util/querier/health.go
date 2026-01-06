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
// Uses a single atomic value to avoid race conditions between failure count and timestamp
type endpointHealth struct {
	// Packed state in a single atomic uint64:
	// - Upper 32 bits: failure count (int32)
	// - Lower 32 bits: timestamp in seconds since epoch (uint32, good until year 2106)
	state atomic.Uint64
}

// packState combines failure count and timestamp into a single uint64
func packState(failures int32, timestampSec uint32) uint64 {
	return (uint64(failures) << 32) | uint64(timestampSec)
}

// unpackState extracts failure count and timestamp from state
func unpackState(state uint64) (failures int32, timestampSec uint32) {
	failures = int32(state >> 32)
	timestampSec = uint32(state & 0xFFFFFFFF)
	return
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
	// Reset to zero failures, timestamp doesn't matter
	h.state.Store(0)
}

// recordEndpointFailure increments failure count and updates last failure time atomically
func recordEndpointFailure(endpoint string) {
	h := getEndpointHealth(endpoint)
	now := uint32(time.Now().Unix())

	// Use compare-and-swap to atomically update both failure count and timestamp
	for {
		oldState := h.state.Load()
		oldFailures, _ := unpackState(oldState)
		newFailures := oldFailures + 1
		newState := packState(newFailures, now)

		if h.state.CompareAndSwap(oldState, newState) {
			break
		}
		// CAS failed due to concurrent update, retry
	}
}

// isEndpointHealthy checks if an endpoint is healthy enough to use
func isEndpointHealthy(endpoint string) bool {
	h := getEndpointHealth(endpoint)
	state := h.state.Load()
	failures, timestampSec := unpackState(state)

	// If below failure threshold, endpoint is healthy
	if failures < failureThreshold {
		return true
	}

	// At or above threshold - check if recovery timeout has passed
	if timestampSec == 0 {
		// Should not happen, but treat as unhealthy to be safe
		return false
	}

	timeSinceFailure := time.Since(time.Unix(int64(timestampSec), 0))
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
