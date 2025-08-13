package plugins

import (
	"regexp"
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
	metrics.DBQueriesTotal().WithLabelValues(operation, status).Inc()
	metrics.DBQueryDuration().WithLabelValues(operation, tableName).Observe(duration)
	
	// Track rows affected for write operations
	if operation != "SELECT" && db.RowsAffected >= 0 {
		metrics.DBRowsAffected().WithLabelValues(operation).Observe(float64(db.RowsAffected))
	}
}

// getOperationType extracts the operation type from the SQL statement
func getOperationType(db *gorm.DB) string {
	if db.Statement == nil || db.Statement.SQL.String() == "" {
		return "UNKNOWN"
	}
	
	sql := strings.ToUpper(strings.TrimSpace(db.Statement.SQL.String()))
	
	if strings.HasPrefix(sql, "SELECT") {
		return "SELECT"
	} else if strings.HasPrefix(sql, "INSERT") {
		return "INSERT"
	} else if strings.HasPrefix(sql, "UPDATE") {
		return "UPDATE"
	} else if strings.HasPrefix(sql, "DELETE") {
		return "DELETE"
	} else if strings.HasPrefix(sql, "CREATE") {
		return "CREATE"
	} else if strings.HasPrefix(sql, "ALTER") {
		return "ALTER"
	} else if strings.HasPrefix(sql, "DROP") {
		return "DROP"
	}
	
	return "OTHER"
}

// getTableName extracts the table name from the GORM statement
func getTableName(db *gorm.DB) string {
	if db.Statement == nil {
		return "unknown"
	}
	
	if db.Statement.Table != "" {
		return db.Statement.Table
	}
	
	// Fallback: extract from SQL using regex
	if db.Statement.SQL.String() != "" {
		tableName := extractTableFromSQL(db.Statement.SQL.String())
		if tableName != "" {
			return tableName
		}
	}
	
	return "unknown"
}

// extractTableFromSQL extracts table name from SQL statement using regex
func extractTableFromSQL(sql string) string {
	// Common patterns for extracting table names
	patterns := []string{
		`(?i)FROM\s+["\x60]?(\w+)["\x60]?`,           // SELECT ... FROM table
		`(?i)INSERT\s+INTO\s+["\x60]?(\w+)["\x60]?`,  // INSERT INTO table
		`(?i)UPDATE\s+["\x60]?(\w+)["\x60]?`,         // UPDATE table
		`(?i)DELETE\s+FROM\s+["\x60]?(\w+)["\x60]?`,  // DELETE FROM table
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(sql)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	
	return ""
}