package common

import (
	"database/sql"
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

	// When hasFilters is true, should use regular COUNT
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))

	// Create query with table context
	query := db.Model(&types.CollectedTx{})
	result, err := GetOptimizedCount(query, strategy, true)

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
	result, err := GetOptimizedCount(query, strategy, false)

	assert.NoError(t, err)
	assert.Equal(t, int64(12345), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_NoFilters_PgClassOptimization(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedNft{}

	// When hasFilters is false and strategy uses pg_class, should use pg_class optimization
	expectedSQL := `SELECT CASE\s+WHEN reltuples >= 0 THEN reltuples::BIGINT\s+ELSE 0\s+END\s+FROM pg_class\s+WHERE relname = \$1`
	mock.ExpectQuery(expectedSQL).
		WithArgs("nft").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50000))

	// Create query with table context
	query := db.Model(&types.CollectedNft{})
	result, err := GetOptimizedCount(query, strategy, false)

	assert.NoError(t, err)
	assert.Equal(t, int64(50000), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_PgClassFallback(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedNft{}

	// First query returns 0, triggering fallback to regular COUNT
	expectedSQL := `SELECT CASE\s+WHEN reltuples >= 0 THEN reltuples::BIGINT\s+ELSE 0\s+END\s+FROM pg_class\s+WHERE relname = \$1`
	mock.ExpectQuery(expectedSQL).
		WithArgs("nft").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Fallback to regular COUNT
	mock.ExpectQuery(`SELECT count\(\*\) FROM "nft"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1000))

	// Create query with table context
	query := db.Model(&types.CollectedNft{})
	result, err := GetOptimizedCount(query, strategy, false)

	assert.NoError(t, err)
	assert.Equal(t, int64(1000), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_UnsupportedStrategy(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	// Create a mock strategy that doesn't support fast count
	strategy := &mockUnsupportedStrategy{}

	// Should fallback to regular COUNT
	mock.ExpectQuery(`SELECT count\(\*\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(500))

	// Create query with table context
	query := db.Table("mock_table")
	result, err := GetOptimizedCount(query, strategy, false)

	assert.NoError(t, err)
	assert.Equal(t, int64(500), result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOptimizedCount_DatabaseError(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	strategy := types.CollectedTx{}

	// Simulate database error
	mock.ExpectQuery(`SELECT count\(\*\) FROM "tx"`).
		WillReturnError(sql.ErrConnDone)

	// Create query with table context
	query := db.Model(&types.CollectedTx{})
	result, err := GetOptimizedCount(query, strategy, true)

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

// Benchmark tests for optimization strategies
//
//nolint:dupl
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
		_, _ = GetOptimizedCount(query, strategy, true)
	}
}

//nolint:dupl
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
		_, _ = GetOptimizedCount(query, strategy, false)
	}
}
