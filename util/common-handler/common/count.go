package common

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func GetCountWithTimeout(countQuery *gorm.DB, countTotal bool) (int64, error) {
	var total int64

	if !countTotal {
		return 0, nil
	}

	// Use a transaction with statement_timeout to avoid connection corruption
	if err := countQuery.Transaction(func(tx *gorm.DB) error {
		// Set timeout only for this transaction
		if err := tx.Exec("SET LOCAL statement_timeout = '5s'").Error; err != nil {
			return err
		}

		countErr := tx.Count(&total).Error

		// Reset timeout before committing to prevent leakage to parent transaction
		if resetErr := tx.Exec("RESET statement_timeout").Error; resetErr != nil {
			// Log the reset error but don't override the count error
			if countErr == nil {
				return resetErr
			}
		}

		return countErr
	}); err != nil {
		// Check for statement timeout
		if strings.Contains(err.Error(), "statement timeout") {
			return -1, nil
		}
		return 0, err
	}

	return total, nil
}

// CountOptimizer provides optimized COUNT operations
type CountOptimizer interface {
	GetOptimizedCount(query *gorm.DB, hasFilters bool) (int64, error)
}

// Generic optimized COUNT implementation
func GetOptimizedCount(db *gorm.DB, strategy types.FastCountStrategy, hasFilters, countTotal bool) (int64, error) {
	if hasFilters || !strategy.SupportsFastCount() {
		// Use regular COUNT when filters exist or fast count not supported
		return GetCountWithTimeout(db, countTotal)
	}

	// Use optimization strategy
	switch strategy.GetOptimizationType() {
	case types.CountOptimizationTypeMax:
		return getCountByMax(db, strategy.GetOptimizationField(), countTotal)

	case types.CountOptimizationTypePgClass:
		return getCountByPgClass(db, strategy.TableName(), countTotal)

	default:
		// Fallback to regular COUNT
		return GetCountWithTimeout(db, countTotal)
	}
}

// validFieldName validates that field name only contains safe characters
var validFieldName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// fieldValidationCache caches field validation results to avoid repeated regex evaluation
var fieldValidationCache sync.Map

// isValidFieldName checks if field name is valid, using cache to avoid repeated regex evaluation
func isValidFieldName(field string) bool {
	// Check cache first
	if cached, found := fieldValidationCache.Load(field); found {
		return cached.(bool)
	}

	// Validate and cache the result
	isValid := validFieldName.MatchString(field)
	fieldValidationCache.Store(field, isValid)
	return isValid
}

// Helper: Get count using MAX(field) for sequential fields
func getCountByMax(db *gorm.DB, field string, countTotal bool) (int64, error) {
	if !countTotal {
		return 0, nil
	}

	// Validate field name to prevent SQL injection
	if !isValidFieldName(field) {
		return 0, fmt.Errorf("invalid field name: %s", field)
	}

	var maxValue int64
	query := fmt.Sprintf("COALESCE(MAX(%s), 0)", field)

	// Safely extract table name from the statement
	var tableName string
	if db != nil && db.Statement != nil {
		tableName = db.Statement.Table
	}

	if tableName == "" {
		// When no table name is available, use Session to create clean query
		err := db.Session(&gorm.Session{}).Select(query).Scan(&maxValue).Error
		return maxValue, err
	}

	// Use Session with explicit Table to ensure clean query without Model fields
	err := db.Session(&gorm.Session{}).Table(tableName).Select(query).Scan(&maxValue).Error
	return maxValue, err
}

// Helper: Get count using PostgreSQL statistics
func getCountByPgClass(db *gorm.DB, tableName string, countTotal bool) (int64, error) {
	if !countTotal {
		return 0, nil
	}

	var total int64
	err := db.Raw(`
		SELECT COALESCE(reltuples, 0)::BIGINT
		FROM pg_class
		WHERE oid = to_regclass(?)::oid
	`, tableName).Scan(&total).Error

	if err != nil || total == 0 {
		// Fallback to regular COUNT
		return GetCountWithTimeout(db.Table(tableName), countTotal)
	}

	return total, err
}

// Helper: Detect if query has WHERE clauses (simple heuristic)
func HasFilters(conditions ...bool) bool {
	for _, condition := range conditions {
		if condition {
			return true
		}
	}
	return false
}
