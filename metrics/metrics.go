package metrics

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/initia-labs/rollytics/config"
)

// DBStatsProvider interface for getting database statistics
type DBStatsProvider interface {
	GetDBStats() (*sql.DBStats, error)
}

// Metrics contains all metric groups
type Metrics struct {
	HTTP        *HTTPMetrics
	Database    *DatabaseMetrics
	Indexer     *IndexerMetrics
	ExternalAPI *ExternalAPIMetrics
	Error       *ErrorMetrics
}

var (
	// Global registry and metrics
	registry *prometheus.Registry
	metrics  *Metrics

	// Global DB stats updater
	dbStatsUpdater *DBStatsUpdater
)

// MetricsServer represents the Prometheus metrics HTTP server
type MetricsServer struct {
	server *http.Server
	logger *slog.Logger
	cfg    *config.MetricsConfig
}

// Init initializes the Prometheus metrics registry and registers all metrics
func Init() {
	registry = prometheus.NewRegistry()

	// Create metric groups
	metrics = &Metrics{
		HTTP:        NewHTTPMetrics(),
		Database:    NewDatabaseMetrics(),
		Indexer:     NewIndexerMetrics(),
		ExternalAPI: NewExternalAPIMetrics(),
		Error:       NewErrorMetrics(),
	}

	// Register all metric groups
	metrics.HTTP.Register(registry)
	metrics.Database.Register(registry)
	metrics.Indexer.Register(registry)
	metrics.ExternalAPI.Register(registry)
	metrics.Error.Register(registry)

	// Add Go runtime metrics
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Start endpoint tracking for detailed analysis
	StartEndpointTracking()
}

// NewServer creates a new metrics server
func NewServer(cfg *config.Config, logger *slog.Logger) *MetricsServer {
	metricsConfig := cfg.GetMetricsConfig()

	// Ensure metrics subsystem is initialized
	if registry == nil || metrics == nil {
		Init()
	}

	mux := http.NewServeMux()
	mux.Handle(metricsConfig.Path, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	server := &http.Server{
		Addr:              ":" + metricsConfig.Port,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	return &MetricsServer{
		server: server,
		logger: logger.With("component", "metrics"),
		cfg:    metricsConfig,
	}
}

// Start starts the metrics server
func (m *MetricsServer) Start() error {
	if !m.cfg.Enabled {
		m.logger.Info("metrics server disabled")
		return nil
	}

	m.logger.Info("starting metrics server",
		slog.String("addr", m.server.Addr),
		slog.String("path", m.cfg.Path))

	return m.server.ListenAndServe()
}

// Shutdown gracefully shuts down the metrics server
func (m *MetricsServer) Shutdown(ctx context.Context) error {
	if !m.cfg.Enabled {
		return nil
	}

	m.logger.Info("shutting down metrics server")
	return m.server.Shutdown(ctx)
}

// GetRegistry returns the Prometheus registry
func GetRegistry() *prometheus.Registry {
	return registry
}

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	return metrics
}

// Legacy accessor functions for backward compatibility
func HTTPRequestsTotal() *prometheus.CounterVec {
	return metrics.HTTP.RequestsTotal
}

func HTTPRequestDuration() *prometheus.HistogramVec {
	return metrics.HTTP.RequestDuration
}

func DBConnectionsActive() prometheus.Gauge {
	return metrics.Database.ConnectionsActive
}

func DBQueriesTotal() *prometheus.CounterVec {
	return metrics.Database.QueriesTotal
}

func BlocksProcessedTotal() prometheus.Counter {
	return metrics.Indexer.BlocksProcessedTotal
}

func CurrentBlockHeight() prometheus.Gauge {
	return metrics.Indexer.CurrentBlockHeight
}

func ExternalAPIRequestsTotal() *prometheus.CounterVec {
	return metrics.ExternalAPI.RequestsTotal
}

func RateLimitHitsTotal() *prometheus.CounterVec {
	return metrics.ExternalAPI.RateLimitHitsTotal
}

func ExternalAPILatency() *prometheus.HistogramVec {
	return metrics.ExternalAPI.Latency
}

func ConcurrentRequestsActive() prometheus.Gauge {
	return metrics.ExternalAPI.ConcurrentActive
}

func SemaphoreWaitDuration() prometheus.Histogram {
	return metrics.ExternalAPI.SemaphoreWaitDuration
}

func DBQueryDuration() *prometheus.HistogramVec {
	return metrics.Database.QueryDuration
}

func DBRowsAffected() *prometheus.HistogramVec {
	return metrics.Database.RowsAffected
}

// StartDBStatsUpdater starts periodic database statistics collection
func StartDBStatsUpdater(provider DBStatsProvider, logger *slog.Logger) {
	if dbStatsUpdater != nil {
		return // Already started
	}

	dbStatsUpdater = NewDBStatsUpdater(provider, logger, metrics.Database)
	dbStatsUpdater.Start()
}

// StopDBStatsUpdater stops the database statistics collection
func StopDBStatsUpdater() {
	if dbStatsUpdater != nil {
		dbStatsUpdater.Stop()
		dbStatsUpdater = nil
	}
}
