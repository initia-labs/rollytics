package metrics

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"sync"
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

	// Singleton initialization
	initOnce sync.Once

	// Chain identifier for metrics labeling
	rollupChainId string
)

// constLabels returns the constant labels to be added to all metrics
func constLabels() prometheus.Labels {
	if rollupChainId == "" {
		return nil
	}
	return prometheus.Labels{"rollup_chain_id": rollupChainId}
}

// MetricsServer represents the Prometheus metrics HTTP server
type MetricsServer struct {
	server *http.Server
	logger *slog.Logger
	cfg    *config.MetricsConfig
}

// Init initializes the Prometheus metrics registry and registers all metrics
// This function is safe to call multiple times - it will only initialize once
// chainId is used as the rollup_chain_id label value for all metrics
func Init(chainId string) {
	initOnce.Do(func() {
		rollupChainId = chainId
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
	})
}

// NewServer creates a new metrics server
func NewServer(cfg *config.Config, logger *slog.Logger) *MetricsServer {
	metricsConfig := cfg.GetMetricsConfig()

	// Ensure metrics subsystem is initialized
	if registry == nil || metrics == nil {
		Init(cfg.GetChainId())
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
	StopDBStatsUpdater()
	StopEndpointTracking()
	return m.server.Shutdown(ctx)
}

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	return metrics
}

// HTTPMetrics returns the HTTP metrics group
func (m *Metrics) HTTPMetrics() *HTTPMetrics {
	return m.HTTP
}

// DatabaseMetrics returns the Database metrics group
func (m *Metrics) DatabaseMetrics() *DatabaseMetrics {
	return m.Database
}

// IndexerMetrics returns the Indexer metrics group
func (m *Metrics) IndexerMetrics() *IndexerMetrics {
	return m.Indexer
}

// ExternalAPIMetrics returns the ExternalAPI metrics group
func (m *Metrics) ExternalAPIMetrics() *ExternalAPIMetrics {
	return m.ExternalAPI
}

// ErrorMetrics returns the Error metrics group
func (m *Metrics) ErrorMetrics() *ErrorMetrics {
	return m.Error
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
