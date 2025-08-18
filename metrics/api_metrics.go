package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	LatencyBuckets   = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	SemaphoreBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}
)

// ExternalAPIMetrics groups external API-related metrics
type ExternalAPIMetrics struct {
	RequestsTotal         *prometheus.CounterVec
	Latency               *prometheus.HistogramVec
	ConcurrentActive      prometheus.Gauge
	SemaphoreWaitDuration prometheus.Histogram
	RateLimitHitsTotal    *prometheus.CounterVec
}

// NewExternalAPIMetrics creates and returns external API metrics
func NewExternalAPIMetrics() *ExternalAPIMetrics {
	return &ExternalAPIMetrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_external_api_requests_total",
				Help: "Total number of external API requests",
			},
			[]string{"endpoint", "status_code"},
		),
		Latency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rollytics_external_api_latency_seconds",
				Help:    "External API request latency in seconds",
				Buckets: LatencyBuckets,
			},
			[]string{"endpoint"},
		),
		ConcurrentActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_concurrent_requests_active",
				Help: "Number of currently active external API requests",
			},
		),
		SemaphoreWaitDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "rollytics_semaphore_wait_duration_seconds",
				Help:    "Time spent waiting for semaphore acquisition",
				Buckets: SemaphoreBuckets,
			},
		),
		RateLimitHitsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_rate_limit_hits_total",
				Help: "Total number of rate limit hits (429 errors)",
			},
			[]string{"endpoint"},
		),
	}
}

// Register registers all external API metrics with the given registry
func (e *ExternalAPIMetrics) Register(reg *prometheus.Registry) {
	reg.MustRegister(
		e.RequestsTotal,
		e.Latency,
		e.ConcurrentActive,
		e.SemaphoreWaitDuration,
		e.RateLimitHitsTotal,
	)
}
