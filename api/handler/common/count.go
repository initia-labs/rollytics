package common

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

// CountOptimizer provides optimized COUNT operations
type CountOptimizer interface {
	GetOptimizedCount(query *gorm.DB, hasFilters bool) (int64, error)
}

// Generic optimized COUNT implementation
func GetOptimizedCount(db *gorm.DB, strategy types.FastCountStrategy, hasFilters bool) (int64, error) {
	if hasFilters || !strategy.SupportsFastCount() {
		// Use regular COUNT when filters exist or fast count not supported
		var total int64
		return total, db.Count(&total).Error
	}

	// Use optimization strategy
	switch strategy.GetOptimizationType() {
	case types.CountOptimizationTypeMax:
		return getCountByMax(db, strategy.GetOptimizationField())

	case types.CountOptimizationTypePgClass:
		return getCountByPgClass(db, strategy.TableName())

	default:
		// Fallback to regular COUNT
		var total int64
		return total, db.Count(&total).Error
	}
}

// Helper: Get count using MAX(field) for sequential fields
func getCountByMax(db *gorm.DB, field string) (int64, error) {
	var maxValue int64
	query := fmt.Sprintf("COALESCE(MAX(%s), 0)", field)
	err := db.Select(query).Scan(&maxValue).Error
	return maxValue, err
}

// Helper: Get count using PostgreSQL statistics
func getCountByPgClass(db *gorm.DB, tableName string) (int64, error) {
	var total int64
	err := db.Raw(`
		SELECT CASE 
			WHEN reltuples >= 0 THEN reltuples::BIGINT
			ELSE 0 
		END
		FROM pg_class 
		WHERE relname = ?
	`, tableName).Scan(&total).Error

	if err != nil || total == 0 {
		// Fallback to regular COUNT
		return total, db.Count(&total).Error
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
