package plugins

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/metrics"
)

// MetricsPlugin is a GORM plugin that tracks database query metrics
type MetricsPlugin struct{}

func NewMetricsPlugin() *MetricsPlugin {
	return &MetricsPlugin{}
}

func (p *MetricsPlugin) Name() string {
	return "MetricsPlugin"
}

func (p *MetricsPlugin) Initialize(db *gorm.DB) error {
	// Register callbacks for tracking query metrics
	err := db.Callback().Query().Before("*").Register("metrics:before_query", p.beforeQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Query().After("*").Register("metrics:after_query", p.afterQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Create().Before("*").Register("metrics:before_create", p.beforeQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Create().After("*").Register("metrics:after_create", p.afterQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Update().Before("*").Register("metrics:before_update", p.beforeQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Update().After("*").Register("metrics:after_update", p.afterQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Delete().Before("*").Register("metrics:before_delete", p.beforeQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Delete().After("*").Register("metrics:after_delete", p.afterQuery)
	if err != nil {
		return err
	}

	return nil
}

func (p *MetricsPlugin) beforeQuery(db *gorm.DB) {
	db.Set("metrics:start_time", time.Now())
}

func (p *MetricsPlugin) afterQuery(db *gorm.DB) {
	startTime, exists := db.Get("metrics:start_time")
	if !exists {
		return
	}

	start, ok := startTime.(time.Time)
	if !ok {
		return
	}

	duration := time.Since(start).Seconds()

	// Determine operation type and table name
	operation := getOperationType(db)
	tableName := getTableName(db)

	// Determine status
	status := "success"
	if db.Error != nil {
		status = "error"
	}

	// Track metrics
	dbMetrics := metrics.GetMetrics().DatabaseMetrics()
	dbMetrics.QueriesTotal.WithLabelValues(operation, status).Inc()
	dbMetrics.QueryDuration.WithLabelValues(operation, tableName).Observe(duration)

	// Track rows affected for write operations
	if operation != "SELECT" && db.RowsAffected >= 0 {
		dbMetrics.RowsAffected.WithLabelValues(operation).Observe(float64(db.RowsAffected))
	}
}

// getOperationType extracts the operation type from the SQL statement
func getOperationType(db *gorm.DB) string {
	if db.Statement == nil || db.Statement.SQL.String() == "" {
		return "UNKNOWN"
	}

	sql := strings.TrimSpace(db.Statement.SQL.String())
	if sql == "" {
		return "UNKNOWN"
	}

	// Find the first word (operation type)
	spaceIndex := strings.IndexByte(sql, ' ')
	if spaceIndex == -1 {
		spaceIndex = len(sql)
	}

	firstWord := strings.ToUpper(sql[:spaceIndex])

	switch firstWord {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP":
		return firstWord
	default:
		return "OTHER"
	}
}

// getTableName extracts the table name from the GORM statement
func getTableName(db *gorm.DB) string {
	if db.Statement == nil {
		return "unknown"
	}

	if db.Statement.Table != "" {
		return db.Statement.Table
	}

	return "unknown"
}
