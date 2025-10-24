package common

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func TestHasFilters(t *testing.T) {
	tests := []struct {
		name       string
		conditions []bool
		expected   bool
	}{
		{
			name:       "No conditions",
			conditions: []bool{},
			expected:   false,
		},
		{
			name:       "All false conditions",
			conditions: []bool{false, false, false},
			expected:   false,
		},
		{
			name:       "Some true conditions",
			conditions: []bool{false, true, false},
			expected:   true,
		},
		{
			name:       "All true conditions",
			conditions: []bool{true, true, true},
			expected:   true,
		},
		{
			name:       "Single true condition",
			conditions: []bool{true},
			expected:   true,
		},
		{
			name:       "Single false condition",
			conditions: []bool{false},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasFilters(tt.conditions...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// setupMockDB creates a mock database connection for testing
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create gorm DB: %v", err)
	}

	cleanup := func() {
		_ = sqlDB.Close()
	}

	return gormDB, mock, cleanup
}

func TestGetOptimizedCount_WithFilters(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedTx{}

	// When hasFilters is true, should use regular COUNT with transaction
	mock.ExpectBegin()
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	// Create query with table context
	query := db.Model(&types.CollectedTx{})
	result, err := GetOptimizedCount(query, strategy, true, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(100), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_NoFilters_MaxOptimization(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedTx{}

	// When hasFilters is false, should use MAX optimization
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "tx"`).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(12345))

	// Create query with table context
	query := db.Model(&types.CollectedTx{})
	result, err := GetOptimizedCount(query, strategy, false, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(12345), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetOptimizedCount_ModelWithMaxOptimization tests the GROUP BY issue fix
// This test ensures that when a query with Model() is passed to getCountByMax,
// it properly handles the query without causing GROUP BY errors
func TestGetOptimizedCount_ModelWithMaxOptimization(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	t.Run("CollectedTx with Model - GROUP BY fix", func(t *testing.T) {
		strategy := types.CollectedTx{}

		// The fix should extract table name and use Session + Table
		// This should generate clean SQL without Model fields
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "tx"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(999))

		// Simulate what happens in GetTxs: Model is applied to the query
		query := db.Model(&types.CollectedTx{})
		result, err := GetOptimizedCount(query, strategy, false, true)

		assert.NoError(t, err)
		assert.Equal(t, int64(999), result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CollectedEvmTx with Model - GROUP BY fix", func(t *testing.T) {
		strategy := types.CollectedEvmTx{}

		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "evm_tx"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(888))

		query := db.Model(&types.CollectedEvmTx{})
		result, err := GetOptimizedCount(query, strategy, false, true)

		assert.NoError(t, err)
		assert.Equal(t, int64(888), result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CollectedEvmInternalTx with Model - GROUP BY fix", func(t *testing.T) {
		strategy := types.CollectedEvmInternalTx{}

		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "evm_internal_tx"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(777))

		query := db.Model(&types.CollectedEvmInternalTx{})
		result, err := GetOptimizedCount(query, strategy, false, true)

		assert.NoError(t, err)
		assert.Equal(t, int64(777), result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CollectedBlock with Model - GROUP BY fix", func(t *testing.T) {
		strategy := types.CollectedBlock{}

		// Block uses height field instead of sequence
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(height\), 0\) FROM "block"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(666))

		query := db.Model(&types.CollectedBlock{})
		result, err := GetOptimizedCount(query, strategy, false, true)

		assert.NoError(t, err)
		assert.Equal(t, int64(666), result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestGetCountByMax_DirectCall tests getCountByMax function directly
func TestGetCountByMax_DirectCall(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	t.Run("With nil Statement - should use valid table context", func(t *testing.T) {
		// Use Table() to provide valid table context instead of expecting invalid SQL
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(test_field\), 0\) FROM "test_table"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(9184))

		query := db.Table("test_table")
		result, err := getCountByMax(query, "test_field", true)
		assert.NoError(t, err)
		assert.Equal(t, int64(9184), result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("With Model applied - should use Session and Table", func(t *testing.T) {
		// Expect clean SQL without Model fields
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "tx"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(555))

		// Apply Model to simulate the problematic scenario
		query := db.Model(&types.CollectedTx{})

		// Statement.Table should be set when Model is used
		_ = query.Statement.Parse(&types.CollectedTx{})

		result, err := getCountByMax(query, "sequence", true)

		assert.NoError(t, err)
		assert.Equal(t, int64(555), result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Without Model - should use table context", func(t *testing.T) {
		// When no Model is applied but Table is specified, should use proper FROM clause
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(id\), 0\) FROM "some_table"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(444))

		// Query without Model but with Table
		query := db.Table("some_table")
		result, err := getCountByMax(query, "id", true)

		assert.NoError(t, err)
		assert.Equal(t, int64(444), result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Invalid field name - should return error", func(t *testing.T) {
		// Test SQL injection prevention with regex validation
		query := db.Model(&types.CollectedTx{})

		// Try with invalid field name containing SQL injection attempt
		result, err := getCountByMax(query, "sequence; DROP TABLE users", true)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid field name")
		assert.Equal(t, int64(0), result)

		// No SQL query should be executed
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGetOptimizedCount_NoFilters_PgClassOptimization(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedNft{}

	// When hasFilters is false and strategy uses pg_class, should use pg_class optimization
	expectedSQL := `SELECT COALESCE\(reltuples, 0\)::BIGINT\s+FROM pg_class\s+WHERE oid = to_regclass\(\$1\)::oid`
	mock.ExpectQuery(expectedSQL).
		WithArgs("nft").
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(50000))

	// Create query with table context
	query := db.Model(&types.CollectedNft{})
	result, err := GetOptimizedCount(query, strategy, false, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(50000), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_PgClassFallback(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedNft{}

	// First query returns 0, triggering fallback to regular COUNT
	expectedSQL := `SELECT COALESCE\(reltuples, 0\)::BIGINT\s+FROM pg_class\s+WHERE oid = to_regclass\(\$1\)::oid`
	mock.ExpectQuery(expectedSQL).
		WithArgs("nft").
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(0))

	// Fallback to regular COUNT with transaction
	mock.ExpectBegin()
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "nft"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	// Create query with table context
	query := db.Model(&types.CollectedNft{})
	result, err := GetOptimizedCount(query, strategy, false, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(1000), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_UnsupportedStrategy(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Create a mock strategy that doesn't support fast count
	strategy := &mockUnsupportedStrategy{}

	// Should fallback to regular COUNT with transaction
	mock.ExpectBegin()
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT count\(\*\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	// Create query with table context
	query := db.Table("mock_table")
	result, err := GetOptimizedCount(query, strategy, false, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(500), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_DatabaseError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedTx{}

	// Simulate database error with transaction
	mock.ExpectBegin()
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	// Create query with table context
	query := db.Model(&types.CollectedTx{})
	result, err := GetOptimizedCount(query, strategy, true, true)

	assert.Error(t, err)
	assert.Equal(t, int64(0), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Mock strategy that doesn't support fast count for testing
type mockUnsupportedStrategy struct{}

func (m *mockUnsupportedStrategy) TableName() string { return "mock_table" }
func (m *mockUnsupportedStrategy) GetOptimizationType() types.CountOptimizationType {
	return types.CountOptimizationTypeCount
}
func (m *mockUnsupportedStrategy) GetOptimizationField() string { return "" }
func (m *mockUnsupportedStrategy) SupportsFastCount() bool      { return false }

// Integration test with real strategy types
//
//nolint:dupl
func TestGetOptimizedCount_RealStrategies(t *testing.T) {
	tests := []struct {
		name              string
		strategy          types.FastCountStrategy
		expectedTableName string
		expectedOptType   types.CountOptimizationType
		expectedField     string
		supportsFast      bool
	}{
		{
			name:              "CollectedTx",
			strategy:          types.CollectedTx{},
			expectedTableName: "tx",
			expectedOptType:   types.CountOptimizationTypeMax,
			expectedField:     "sequence",
			supportsFast:      true,
		},
		{
			name:              "CollectedEvmTx",
			strategy:          types.CollectedEvmTx{},
			expectedTableName: "evm_tx",
			expectedOptType:   types.CountOptimizationTypeMax,
			expectedField:     "sequence",
			supportsFast:      true,
		},
		{
			name:              "CollectedEvmInternalTx",
			strategy:          types.CollectedEvmInternalTx{},
			expectedTableName: "evm_internal_tx",
			expectedOptType:   types.CountOptimizationTypeMax,
			expectedField:     "sequence",
			supportsFast:      true,
		},
		{
			name:              "CollectedBlock",
			strategy:          types.CollectedBlock{},
			expectedTableName: "block",
			expectedOptType:   types.CountOptimizationTypeMax,
			expectedField:     "height",
			supportsFast:      true,
		},
		{
			name:              "CollectedNftCollection",
			strategy:          types.CollectedNftCollection{},
			expectedTableName: "nft_collection",
			expectedOptType:   types.CountOptimizationTypePgClass,
			expectedField:     "",
			supportsFast:      true,
		},
		{
			name:              "CollectedNft",
			strategy:          types.CollectedNft{},
			expectedTableName: "nft",
			expectedOptType:   types.CountOptimizationTypePgClass,
			expectedField:     "",
			supportsFast:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedTableName, tt.strategy.TableName())
			assert.Equal(t, tt.expectedOptType, tt.strategy.GetOptimizationType())
			assert.Equal(t, tt.expectedField, tt.strategy.GetOptimizationField())
			assert.Equal(t, tt.supportsFast, tt.strategy.SupportsFastCount())
		})
	}
}

func TestCountOptimizationTypes(t *testing.T) {
	// Test that optimization type constants are properly defined
	assert.Equal(t, types.CountOptimizationTypeMax, types.CountOptimizationType(1))
	assert.Equal(t, types.CountOptimizationTypePgClass, types.CountOptimizationType(2))
	assert.Equal(t, types.CountOptimizationTypeCount, types.CountOptimizationType(3))
}

func TestCountOptimizer_Interface(t *testing.T) {
	// Test that CountOptimizer interface is properly defined
	var optimizer CountOptimizer
	assert.Nil(t, optimizer) // Interface should be nil when not implemented
}

// Test optimization logic paths (unit test level)
func TestOptimizationStrategyLogic(t *testing.T) {
	tests := []struct {
		name               string
		strategy           types.FastCountStrategy
		hasFilters         bool
		expectOptimization bool
		expectedOptType    types.CountOptimizationType
	}{
		{
			name:               "TX without filters - should optimize",
			strategy:           types.CollectedTx{},
			hasFilters:         false,
			expectOptimization: true,
			expectedOptType:    types.CountOptimizationTypeMax,
		},
		{
			name:               "TX with filters - should not optimize",
			strategy:           types.CollectedTx{},
			hasFilters:         true,
			expectOptimization: false,
			expectedOptType:    types.CountOptimizationTypeMax,
		},
		{
			name:               "NFT without filters - should optimize",
			strategy:           types.CollectedNft{},
			hasFilters:         false,
			expectOptimization: true,
			expectedOptType:    types.CountOptimizationTypePgClass,
		},
		{
			name:               "NFT with filters - should not optimize",
			strategy:           types.CollectedNft{},
			hasFilters:         true,
			expectOptimization: false,
			expectedOptType:    types.CountOptimizationTypePgClass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test strategy properties
			assert.True(t, tt.strategy.SupportsFastCount())
			assert.Equal(t, tt.expectedOptType, tt.strategy.GetOptimizationType())

			// Test optimization logic
			shouldOptimize := !tt.hasFilters && tt.strategy.SupportsFastCount()
			assert.Equal(t, tt.expectOptimization, shouldOptimize)
		})
	}
}

func TestOptimizationFieldMapping(t *testing.T) {
	tests := []struct {
		name          string
		strategy      types.FastCountStrategy
		expectedField string
		usesField     bool
	}{
		{
			name:          "TX uses sequence field",
			strategy:      types.CollectedTx{},
			expectedField: "sequence",
			usesField:     true,
		},
		{
			name:          "Block uses height field",
			strategy:      types.CollectedBlock{},
			expectedField: "height",
			usesField:     true,
		},
		{
			name:          "NFT Collection uses pg_class (no field)",
			strategy:      types.CollectedNftCollection{},
			expectedField: "",
			usesField:     false,
		},
		{
			name:          "NFT uses pg_class (no field)",
			strategy:      types.CollectedNft{},
			expectedField: "",
			usesField:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := tt.strategy.GetOptimizationField()
			assert.Equal(t, tt.expectedField, field)

			if tt.usesField {
				assert.NotEmpty(t, field, "MAX optimization should have a field")
				assert.Equal(t, types.CountOptimizationTypeMax, tt.strategy.GetOptimizationType())
			} else {
				assert.Empty(t, field, "pg_class optimization should not have a field")
				assert.Equal(t, types.CountOptimizationTypePgClass, tt.strategy.GetOptimizationType())
			}
		})
	}
}

func TestTableNameConsistency(t *testing.T) {
	// Test that TableName() returns consistent values for GORM compatibility
	strategies := []types.FastCountStrategy{
		types.CollectedTx{},
		types.CollectedEvmTx{},
		types.CollectedEvmInternalTx{},
		types.CollectedBlock{},
		types.CollectedNftCollection{},
		types.CollectedNft{},
	}

	expectedNames := []string{
		"tx",
		"evm_tx",
		"evm_internal_tx",
		"block",
		"nft_collection",
		"nft",
	}

	for i, strategy := range strategies {
		t.Run(expectedNames[i], func(t *testing.T) {
			tableName := strategy.TableName()
			assert.Equal(t, expectedNames[i], tableName)
			assert.NotEmpty(t, tableName, "Table name should not be empty")
		})
	}
}

// TestFieldValidationCaching tests that sync.Map caching works for field validation
func TestFieldValidationCaching(t *testing.T) {
	// Clear the cache before testing
	fieldValidationCache.Range(func(key, value interface{}) bool {
		fieldValidationCache.Delete(key)
		return true
	})

	// Test valid field names
	validFields := []string{"sequence", "height", "id", "created_at"}
	for _, field := range validFields {
		// First call should compute and cache
		result1 := isValidFieldName(field)
		assert.True(t, result1, "Field %s should be valid", field)

		// Second call should use cache
		result2 := isValidFieldName(field)
		assert.True(t, result2, "Field %s should be valid on second call", field)
		assert.Equal(t, result1, result2, "Results should be consistent")

		// Verify it's in cache
		cached, found := fieldValidationCache.Load(field)
		assert.True(t, found, "Field %s should be in cache", field)
		assert.True(t, cached.(bool), "Cached value should be true for valid field %s", field)
	}

	// Test invalid field names
	invalidFields := []string{"DROP TABLE", "1invalid", "field;DROP", "field--comment"}
	for _, field := range invalidFields {
		// First call should compute and cache
		result1 := isValidFieldName(field)
		assert.False(t, result1, "Field %s should be invalid", field)

		// Second call should use cache
		result2 := isValidFieldName(field)
		assert.False(t, result2, "Field %s should be invalid on second call", field)
		assert.Equal(t, result1, result2, "Results should be consistent")

		// Verify it's in cache
		cached, found := fieldValidationCache.Load(field)
		assert.True(t, found, "Field %s should be in cache", field)
		assert.False(t, cached.(bool), "Cached value should be false for invalid field %s", field)
	}

	// Test that cache persists values
	assert.True(t, isValidFieldName("sequence"), "sequence should still be valid")
	assert.False(t, isValidFieldName("DROP TABLE"), "DROP TABLE should still be invalid")
}

// BenchmarkFieldValidation benchmarks field validation with and without caching
func BenchmarkFieldValidation(b *testing.B) {
	// Clear cache before benchmarking
	fieldValidationCache.Range(func(key, value interface{}) bool {
		fieldValidationCache.Delete(key)
		return true
	})

	// Benchmark with repeated field names (cache should help)
	b.Run("RepeatedFields", func(b *testing.B) {
		fields := []string{"sequence", "height", "id", "created_at"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			field := fields[i%len(fields)]
			isValidFieldName(field)
		}
	})

	// Benchmark with unique field names (no cache benefit)
	b.Run("UniqueFields", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			field := fmt.Sprintf("field_%d", i)
			isValidFieldName(field)
		}
	})
}

// Benchmark tests for optimization strategies
//

func BenchmarkGetOptimizedCount_WithFilters(b *testing.B) {
	db, mock, cleanup := setupMockDB(&testing.T{})
	defer cleanup()

	strategy := types.CollectedTx{}

	// Setup expectation for benchmark
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := db.Model(&types.CollectedTx{})
		_, _ = GetOptimizedCount(query, strategy, true, true)
	}
}

func BenchmarkGetOptimizedCount_WithoutFilters(b *testing.B) {
	db, mock, cleanup := setupMockDB(&testing.T{})
	defer cleanup()

	strategy := types.CollectedTx{}

	// Setup expectation for benchmark
	for i := 0; i < b.N; i++ {
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(sequence\), 0\) FROM "tx"`).
			WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(12345))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := db.Model(&types.CollectedTx{})
		_, _ = GetOptimizedCount(query, strategy, false, true)
	}
}

// Tests for GetCountWithTimeout function
func TestGetCountWithTimeout_Success(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Expect transaction begin
	mock.ExpectBegin()
	// Expect SET LOCAL statement_timeout
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect COUNT query
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1500))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect transaction commit
	mock.ExpectCommit()

	query := db.Model(&types.CollectedTx{})
	result, err := GetCountWithTimeout(query, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(1500), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCountWithTimeout_StatementTimeout(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Expect transaction begin
	mock.ExpectBegin()
	// Expect SET LOCAL statement_timeout
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect COUNT query to return timeout error
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnError(fmt.Errorf("canceling statement due to statement timeout"))
	// Expect transaction rollback
	mock.ExpectRollback()

	query := db.Model(&types.CollectedTx{})
	result, err := GetCountWithTimeout(query, true)

	assert.NoError(t, err)             // Should not return error for timeout
	assert.Equal(t, int64(-1), result) // Should return -1 for timeout
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCountWithTimeout_DatabaseError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Expect transaction begin
	mock.ExpectBegin()
	// Expect SET LOCAL statement_timeout
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect COUNT query to return database error
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnError(sql.ErrConnDone)
	// Expect transaction rollback
	mock.ExpectRollback()

	query := db.Model(&types.CollectedTx{})
	result, err := GetCountWithTimeout(query, true)

	assert.Error(t, err)
	assert.Equal(t, int64(0), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCountWithTimeout_TimeoutSettingError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Expect transaction begin
	mock.ExpectBegin()
	// Expect SET LOCAL statement_timeout to fail
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnError(sql.ErrConnDone)
	// Expect transaction rollback
	mock.ExpectRollback()

	query := db.Model(&types.CollectedTx{})
	result, err := GetCountWithTimeout(query, true)

	assert.Error(t, err)
	assert.Equal(t, int64(0), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCountWithTimeout_EmptyResult(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Expect transaction begin
	mock.ExpectBegin()
	// Expect SET LOCAL statement_timeout
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect COUNT query to return 0
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect transaction commit
	mock.ExpectCommit()

	query := db.Model(&types.CollectedTx{})
	result, err := GetCountWithTimeout(query, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(0), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCountWithTimeout_WithFilters(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Expect transaction begin
	mock.ExpectBegin()
	// Expect SET LOCAL statement_timeout
	mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect COUNT query with WHERE clause - match the exact SQL pattern
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx" WHERE sequence > \$1`).
		WithArgs(1000).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))
	mock.ExpectExec(`RESET statement_timeout`).WillReturnResult(sqlmock.NewResult(0, 0))
	// Expect transaction commit
	mock.ExpectCommit()

	query := db.Model(&types.CollectedTx{}).Where("sequence > ?", 1000)
	result, err := GetCountWithTimeout(query, true)

	assert.NoError(t, err)
	assert.Equal(t, int64(42), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCountWithTimeout_TimeoutErrorVariations(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	timeoutErrors := []string{
		"canceling statement due to statement timeout",
		"statement timeout",
		"pq: canceling statement due to statement timeout",
	}

	for i, timeoutError := range timeoutErrors {
		t.Run(fmt.Sprint("TimeoutError_", i), func(t *testing.T) {
			// Expect transaction begin
			mock.ExpectBegin()
			// Expect SET LOCAL statement_timeout
			mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
				WillReturnResult(sqlmock.NewResult(0, 0))
			// Expect COUNT query to return timeout error
			mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
				WillReturnError(fmt.Errorf("%s", timeoutError))
			// Expect transaction rollback
			mock.ExpectRollback()

			query := db.Model(&types.CollectedTx{})
			result, err := GetCountWithTimeout(query, true)

			assert.NoError(t, err)             // Should not return error for timeout
			assert.Equal(t, int64(-1), result) // Should return -1 for timeout
		})
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCountWithTimeout_NonTimeoutError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	nonTimeoutErrors := []string{
		"connection refused",
		"table does not exist",
		"syntax error",
		"permission denied",
	}

	for i, nonTimeoutError := range nonTimeoutErrors {
		t.Run(fmt.Sprint("NonTimeoutError_", i), func(t *testing.T) {
			// Expect transaction begin
			mock.ExpectBegin()
			// Expect SET LOCAL statement_timeout
			mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
				WillReturnResult(sqlmock.NewResult(0, 0))
			// Expect COUNT query to return non-timeout error
			mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
				WillReturnError(fmt.Errorf("%s", nonTimeoutError))
			// Expect transaction rollback
			mock.ExpectRollback()

			query := db.Model(&types.CollectedTx{})
			result, err := GetCountWithTimeout(query, true)

			assert.Error(t, err)              // Should return error for non-timeout errors
			assert.Equal(t, int64(0), result) // Should return 0 for errors
			assert.Contains(t, err.Error(), nonTimeoutError)
		})
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Benchmark tests for GetCountWithTimeout
func BenchmarkGetCountWithTimeout_Success(b *testing.B) {
	db, mock, cleanup := setupMockDB(&testing.T{})
	defer cleanup()

	// Setup expectations for benchmark
	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))
		mock.ExpectCommit()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := db.Model(&types.CollectedTx{})
		_, _ = GetCountWithTimeout(query, true)
	}
}

func BenchmarkGetCountWithTimeout_Timeout(b *testing.B) {
	db, mock, cleanup := setupMockDB(&testing.T{})
	defer cleanup()

	// Setup expectations for benchmark
	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectExec(`SET LOCAL statement_timeout = '5s'`).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
			WillReturnError(fmt.Errorf("statement timeout"))
		mock.ExpectRollback()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := db.Model(&types.CollectedTx{})
		_, _ = GetCountWithTimeout(query, true)
	}
}
