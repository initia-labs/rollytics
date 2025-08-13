package metrics

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	DBLatencyBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}
	RowCountBuckets  = []float64{1, 10, 50, 100, 500, 1000, 5000}
)

// DatabaseMetrics groups database-related metrics
type DatabaseMetrics struct {
	ConnectionsActive       prometheus.Gauge
	ConnectionsIdle         prometheus.Gauge
	ConnectionsMaxOpen      prometheus.Gauge
	ConnectionsWaitCount    *prometheus.CounterVec
	ConnectionsWaitDuration prometheus.Histogram
	QueriesTotal            *prometheus.CounterVec
	QueryDuration           *prometheus.HistogramVec
	RowsAffected            *prometheus.HistogramVec
}

// NewDatabaseMetrics creates and returns database metrics
func NewDatabaseMetrics() *DatabaseMetrics {
	return &DatabaseMetrics{
		ConnectionsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_db_connections_active",
				Help: "Number of active database connections",
			},
		),
		ConnectionsIdle: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_db_connections_idle",
				Help: "Number of idle database connections",
			},
		),
		ConnectionsMaxOpen: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "rollytics_db_connections_max_open",
				Help: "Maximum number of open database connections",
			},
		),
		ConnectionsWaitCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_db_connections_wait_count_total",
				Help: "Total number of database connection waits",
			},
			[]string{"status"}, // "timeout", "success"
		),
		ConnectionsWaitDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "rollytics_db_connections_wait_duration_seconds",
				Help:    "Time spent waiting for database connections",
				Buckets: prometheus.DefBuckets,
			},
		),
		QueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rollytics_db_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"operation", "status"},
		),
		QueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rollytics_db_query_duration_seconds",
				Help:    "Database query execution time in seconds",
				Buckets: DBLatencyBuckets,
			},
			[]string{"operation", "table"},
		),
		RowsAffected: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rollytics_db_rows_affected",
				Help:    "Number of rows affected by database operations",
				Buckets: RowCountBuckets,
			},
			[]string{"operation"},
		),
	}
}

// Register registers all database metrics with the given registry
func (d *DatabaseMetrics) Register(reg *prometheus.Registry) {
	reg.MustRegister(
		d.ConnectionsActive,
		d.ConnectionsIdle,
		d.ConnectionsMaxOpen,
		d.ConnectionsWaitCount,
		d.ConnectionsWaitDuration,
		d.QueriesTotal,
		d.QueryDuration,
		d.RowsAffected,
	)
}

// DBStatsUpdater periodically updates database connection metrics
type DBStatsUpdater struct {
	provider DBStatsProvider
	logger   *slog.Logger
	ticker   *time.Ticker
	done     chan struct{}
	metrics  *DatabaseMetrics
}

// NewDBStatsUpdater creates a new database stats updater
func NewDBStatsUpdater(provider DBStatsProvider, logger *slog.Logger, metrics *DatabaseMetrics) *DBStatsUpdater {
	return &DBStatsUpdater{
		provider: provider,
		logger:   logger.With("component", "db_stats"),
		ticker:   time.NewTicker(10 * time.Second),
		done:     make(chan struct{}),
		metrics:  metrics,
	}
}

// Start starts the database stats updater
func (u *DBStatsUpdater) Start() {
	u.logger.Info("starting database stats updater")

	// Update once immediately
	u.updateStats()

	go u.run()
}

// Stop stops the database stats updater
func (u *DBStatsUpdater) Stop() {
	u.logger.Info("stopping database stats updater")
	u.ticker.Stop()
	close(u.done)
}

// run is the main loop for updating database statistics
func (u *DBStatsUpdater) run() {
	for {
		select {
		case <-u.ticker.C:
			u.updateStats()
		case <-u.done:
			return
		}
	}
}

// updateStats updates the database connection metrics
func (u *DBStatsUpdater) updateStats() {
	stats, err := u.provider.GetDBStats()
	if err != nil {
		u.logger.Error("failed to get database stats", "error", err)
		return
	}

	// Update Prometheus metrics
	u.metrics.ConnectionsActive.Set(float64(stats.InUse))
	u.metrics.ConnectionsIdle.Set(float64(stats.Idle))
	u.metrics.ConnectionsMaxOpen.Set(float64(stats.MaxOpenConnections))

	// Update counters (these are cumulative)
	u.metrics.ConnectionsWaitCount.WithLabelValues("total").Add(float64(stats.WaitCount))
	if stats.WaitDuration > 0 {
		u.metrics.ConnectionsWaitDuration.Observe(stats.WaitDuration.Seconds())
	}

	u.logger.Debug("updated database stats",
		"active", stats.InUse,
		"idle", stats.Idle,
		"max_open", stats.MaxOpenConnections,
		"wait_count", stats.WaitCount)
}
