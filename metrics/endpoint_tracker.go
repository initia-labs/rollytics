package metrics

import (
	"sort"
	"sync"
	"time"
)

// EndpointTracker tracks endpoint performance for detailed monitoring
type EndpointTracker struct {
	mu        sync.RWMutex
	endpoints map[string]*EndpointStats
	ticker    *time.Ticker
	done      chan struct{}
}

// EndpointStats holds statistics for an endpoint
type EndpointStats struct {
	Path      string
	Durations []float64
	Count     int64
	LastSeen  time.Time
}

var globalTracker *EndpointTracker

// StartEndpointTracking initializes and starts the endpoint tracker
func StartEndpointTracking() {
	if globalTracker != nil {
		return
	}

	globalTracker = &EndpointTracker{
		endpoints: make(map[string]*EndpointStats),
		ticker:    time.NewTicker(5 * time.Minute),
		done:      make(chan struct{}),
	}

	go globalTracker.run()
}

// StopEndpointTracking stops the endpoint tracker
func StopEndpointTracking() {
	if globalTracker != nil {
		close(globalTracker.done)
		globalTracker.ticker.Stop()
		globalTracker = nil
	}
}

// TrackEndpoint records endpoint performance data
func TrackEndpoint(path string, duration float64) {
	if globalTracker == nil {
		return
	}

	globalTracker.mu.Lock()
	defer globalTracker.mu.Unlock()

	stats, exists := globalTracker.endpoints[path]
	if !exists {
		stats = &EndpointStats{
			Path:      path,
			Durations: make([]float64, 0, 1000), // Pre-allocate for performance
		}
		globalTracker.endpoints[path] = stats
	}

	stats.Durations = append(stats.Durations, duration)
	stats.Count++
	stats.LastSeen = time.Now()

	// Keep only recent data (last 1000 requests per endpoint)
	if len(stats.Durations) > 1000 {
		stats.Durations = stats.Durations[len(stats.Durations)-1000:]
	}
}

// run periodically updates the TopEndpoints metrics
func (et *EndpointTracker) run() {
	for {
		select {
		case <-et.ticker.C:
			et.updateTopEndpoints()
		case <-et.done:
			return
		}
	}
}

// updateTopEndpoints calculates and updates the top slow endpoints metric
func (et *EndpointTracker) updateTopEndpoints() {
	et.mu.RLock()
	defer et.mu.RUnlock()

	type endpointMetric struct {
		path string
		p99  float64
	}

	var metrics []endpointMetric
	cutoff := time.Now().Add(-10 * time.Minute) // Only consider recent data

	for path, stats := range et.endpoints {
		if stats.LastSeen.Before(cutoff) || len(stats.Durations) < 10 {
			continue // Skip inactive or low-traffic endpoints
		}

		// Calculate P99
		durations := make([]float64, len(stats.Durations))
		copy(durations, stats.Durations)
		sort.Float64s(durations)

		p99Index := int(float64(len(durations)) * 0.99)
		if p99Index >= len(durations) {
			p99Index = len(durations) - 1
		}

		p99 := durations[p99Index]
		if p99 > 0.1 { // Only track endpoints with P99 > 100ms
			metrics = append(metrics, endpointMetric{path: path, p99: p99})
		}
	}

	// Sort by P99 and keep only top 20 slowest endpoints
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].p99 > metrics[j].p99
	})

	// Clear old metrics
	GetMetrics().HTTP.TopEndpoints.Reset()

	// Update with top slow endpoints (limit to 20 to control cardinality)
	limit := 20
	if len(metrics) < limit {
		limit = len(metrics)
	}

	for i := 0; i < limit; i++ {
		GetMetrics().HTTP.TopEndpoints.WithLabelValues(metrics[i].path).Set(metrics[i].p99)
	}
}