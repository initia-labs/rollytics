package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPLatencyBuckets = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}
)

// HTTPMetrics groups HTTP-related metrics
type HTTPMetrics struct {
	// Core HTTP metrics
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge

	// Error tracking
	ErrorsTotal *prometheus.CounterVec

	// Detailed metrics for troubleshooting (lower cardinality sampling)
	SlowRequests *prometheus.CounterVec
	TopEndpoints *prometheus.GaugeVec
}

// NewHTTPMetrics creates and returns HTTP metrics
func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "handler", "status_class"}, // status_class: 2xx, 3xx, 4xx, 5xx
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rollytics_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: HTTPLatencyBuckets,
			},
			[]string{"method", "handler"},
		),
		RequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
		),
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_http_errors_total",
				Help: "Total number of HTTP errors",
			},
			[]string{"handler", "error_type"},
		),
		SlowRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_http_slow_requests_total",
				Help: "Total number of slow requests (>1s) with full path for debugging",
			},
			[]string{"method", "path", "duration_bucket"}, // "1-2s", "2-5s", "5s+"
		),
		TopEndpoints: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rollytics_http_top_endpoints_duration_p99",
				Help: "P99 duration of top slow endpoints (updated every 5min)",
			},
			[]string{"path"}, // Only for top N slowest endpoints
		),
	}
}

// Register registers all HTTP metrics with the given registry
func (h *HTTPMetrics) Register(reg *prometheus.Registry) {
	reg.MustRegister(
		h.RequestsTotal,
		h.RequestDuration,
		h.RequestsInFlight,
		h.ErrorsTotal,
		h.SlowRequests,
		h.TopEndpoints,
	)
}

// GetStatusClass converts HTTP status code to class (2xx, 3xx, 4xx, 5xx)
func GetStatusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "other"
	}
}

// GetHandlerPattern converts full path to handler pattern
func GetHandlerPattern(path string) string {
	if len(path) == 0 {
		return "root"
	}

	// Common API patterns
	switch {
	case path == "/":
		return "root"
	case strings.HasPrefix(path, "/swagger"):
		return "swagger"
	case path == "/health" || path == "/ping":
		return "health"
	case strings.HasPrefix(path, "/indexer/"):
		// Extract handler type from path like /indexer/blocks, /indexer/txs
		parts := strings.Split(path, "/")
		if len(parts) >= 3 && parts[2] != "" {
			return parts[2] // blocks, txs, nfts, etc.
		}
		return "indexer"
	default:
		return "other"
	}
}

// GetDurationBucket categorizes request duration for slow request tracking
func GetDurationBucket(seconds float64) string {
	switch {
	case seconds < 1:
		return "" // Don't track fast requests
	case seconds < 2:
		return "1-2s"
	case seconds < 5:
		return "2-5s"
	default:
		return "5s+"
	}
}

// ShouldTrackDetailed determines if this request should be tracked in detail
func ShouldTrackDetailed(duration float64, path string) bool {
	// Track if slow OR specific important endpoints
	if duration >= 1.0 {
		return true
	}

	// Always track certain critical endpoints even if fast
	switch {
	case strings.Contains(path, "/latest"):
		return true
	case strings.Contains(path, "/health"):
		return false // Don't spam with health checks
	default:
		return false
	}
}
