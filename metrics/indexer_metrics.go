package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	IndexerLatencyBuckets = []float64{0.1, 0.5, 1, 2.5, 5, 10, 30}
)

// IndexerMetrics groups indexer-related metrics
type IndexerMetrics struct {
	// Core processing metrics
	BlocksProcessedTotal prometheus.Counter
	CurrentBlockHeight   prometheus.Gauge
	BlockProcessingTime  *prometheus.HistogramVec

	// Queue and throughput metrics
	InflightBlocksCount prometheus.Gauge
	ProcessingSpeed     prometheus.Gauge

	// Error tracking
	ProcessingErrors *prometheus.CounterVec
}

// NewIndexerMetrics creates and returns indexer metrics
func NewIndexerMetrics() *IndexerMetrics {
	return &IndexerMetrics{
		BlocksProcessedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "rollytics_blocks_processed_total",
				Help: "Total number of blocks processed",
			},
		),
		CurrentBlockHeight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_current_block_height",
				Help: "Current block height being processed",
			},
		),
		BlockProcessingTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rollytics_block_processing_duration_seconds",
				Help:    "Time spent processing blocks",
				Buckets: IndexerLatencyBuckets,
			},
			[]string{"stage"}, // "scrape", "prepare", "collect"
		),
		InflightBlocksCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_inflight_blocks_count",
				Help: "Number of blocks currently being processed",
			},
		),
		ProcessingSpeed: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_processing_speed_blocks_per_second",
				Help: "Current processing speed in blocks per second",
			},
		),
		ProcessingErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_processing_errors_total",
				Help: "Total number of processing errors",
			},
			[]string{"stage", "error_type"}, // stage: scrape, prepare, collect
		),
	}
}

// Register registers all indexer metrics with the given registry
func (i *IndexerMetrics) Register(reg *prometheus.Registry) {
	reg.MustRegister(
		i.BlocksProcessedTotal,
		i.CurrentBlockHeight,
		i.BlockProcessingTime,
		i.InflightBlocksCount,
		i.ProcessingSpeed,
		i.ProcessingErrors,
	)
}
