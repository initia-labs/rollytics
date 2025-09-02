package tx

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

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

//nolint:dupl
func TestTxHandler_getAccounts(t *testing.T) {
	tests := []struct {
		name           string
		txs            []types.CollectedEvmInternalTx
		expectedQuery  bool
		expectedResult int // number of expected results
	}{
		{
			name:           "Empty input",
			txs:            []types.CollectedEvmInternalTx{},
			expectedQuery:  false,
			expectedResult: 0,
		},
		{
			name: "Invalid IDs only (zero and negative)",
			txs: []types.CollectedEvmInternalTx{
				{FromId: 0, ToId: -1},
				{FromId: -2, ToId: 0},
			},
			expectedQuery:  false,
			expectedResult: 0,
		},
		{
			name: "Valid IDs with deduplication",
			txs: []types.CollectedEvmInternalTx{
				{FromId: 1, ToId: 2},
				{FromId: 2, ToId: 3}, // FromId 2 duplicated
				{FromId: 1, ToId: 4}, // FromId 1 duplicated
			},
			expectedQuery:  true,
			expectedResult: 4, // unique IDs: 1, 2, 3, 4
		},
		{
			name: "Mixed valid and invalid IDs",
			txs: []types.CollectedEvmInternalTx{
				{FromId: 1, ToId: 0},  // ToId invalid
				{FromId: -1, ToId: 2}, // FromId invalid
				{FromId: 3, ToId: 4},  // both valid
			},
			expectedQuery:  true,
			expectedResult: 4, // unique valid IDs: 1, 2, 3, 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()

			handler := &TxHandler{}

			if tt.expectedQuery {
				// Create mock data based on expected result count
				rows := sqlmock.NewRows([]string{"id", "account"})
				for i := 1; i <= tt.expectedResult; i++ {
					rows.AddRow(int64(i), []byte(fmt.Sprintf("account%d", i)))
				}
				mock.ExpectQuery(`SELECT .* FROM "account_dict"`).
					WillReturnRows(rows)
			}

			result, err := handler.getAccounts(db, tt.txs)

			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Verify the result size matches expected unique IDs
			if tt.expectedQuery {
				assert.Equal(t, tt.expectedResult, len(result), "Result size should match expected unique valid IDs")
				assert.NoError(t, mock.ExpectationsWereMet())
			} else {
				assert.Equal(t, 0, len(result), "Result should be empty when no query expected")
			}
		})
	}
}

//nolint:dupl
func TestTxHandler_getHashes(t *testing.T) {
	tests := []struct {
		name           string
		txs            []types.CollectedEvmInternalTx
		expectedQuery  bool
		expectedResult int
	}{
		{
			name:           "Empty input",
			txs:            []types.CollectedEvmInternalTx{},
			expectedQuery:  false,
			expectedResult: 0,
		},
		{
			name: "Invalid hash IDs only",
			txs: []types.CollectedEvmInternalTx{
				{HashId: 0},
				{HashId: -1},
				{HashId: -100},
			},
			expectedQuery:  false,
			expectedResult: 0,
		},
		{
			name: "Valid hash IDs with deduplication",
			txs: []types.CollectedEvmInternalTx{
				{HashId: 1},
				{HashId: 2},
				{HashId: 1}, // duplicate
				{HashId: 3},
				{HashId: 2}, // duplicate
			},
			expectedQuery:  true,
			expectedResult: 3, // unique IDs: 1, 2, 3
		},
		{
			name: "Mixed valid and invalid hash IDs",
			txs: []types.CollectedEvmInternalTx{
				{HashId: 1},
				{HashId: 0}, // invalid
				{HashId: 2},
				{HashId: -1}, // invalid
				{HashId: 3},
			},
			expectedQuery:  true,
			expectedResult: 3, // unique valid IDs: 1, 2, 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, cleanup := setupMockDB(t)
			defer cleanup()

			handler := &TxHandler{}

			if tt.expectedQuery {
				// Create mock data based on expected result count
				rows := sqlmock.NewRows([]string{"id", "hash"})
				for i := 1; i <= tt.expectedResult; i++ {
					rows.AddRow(int64(i), []byte(fmt.Sprintf("hash%d", i)))
				}
				mock.ExpectQuery(`SELECT .* FROM "evm_tx_hash_dict"`).
					WillReturnRows(rows)
			}

			result, err := handler.getHashes(db, tt.txs)

			assert.NoError(t, err)
			assert.NotNil(t, result)

			// Verify the result size matches expected unique IDs
			if tt.expectedQuery {
				assert.Equal(t, tt.expectedResult, len(result), "Result size should match expected unique valid IDs")
				assert.NoError(t, mock.ExpectationsWereMet())
			} else {
				assert.Equal(t, 0, len(result), "Result should be empty when no query expected")
			}
		})
	}
}

func TestTxHandler_getAccountsWithActualData(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	handler := &TxHandler{}

	// Test with actual mock data
	txs := []types.CollectedEvmInternalTx{
		{FromId: 1, ToId: 2},
		{FromId: 2, ToId: 3},
		{FromId: 1, ToId: 4}, // FromId 1 duplicated
	}

	// Mock the database response
	rows := sqlmock.NewRows([]string{"id", "account"}).
		AddRow(1, []byte("account1")).
		AddRow(2, []byte("account2")).
		AddRow(3, []byte("account3")).
		AddRow(4, []byte("account4"))

	mock.ExpectQuery(`SELECT .* FROM "account_dict" WHERE id IN \(\$1,\$2,\$3,\$4\)`).
		WillReturnRows(rows)

	result, err := handler.getAccounts(db, txs)

	assert.NoError(t, err)
	assert.Equal(t, 4, len(result)) // Should have 4 unique accounts
	assert.Equal(t, []byte("account1"), result[1])
	assert.Equal(t, []byte("account2"), result[2])
	assert.Equal(t, []byte("account3"), result[3])
	assert.Equal(t, []byte("account4"), result[4])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTxHandler_getHashesWithActualData(t *testing.T) {
	db, mock, cleanup := setupMockDB(t)
	defer cleanup()

	handler := &TxHandler{}

	// Test with actual mock data
	txs := []types.CollectedEvmInternalTx{
		{HashId: 1},
		{HashId: 2},
		{HashId: 1}, // duplicate
		{HashId: 3},
	}

	// Mock the database response
	rows := sqlmock.NewRows([]string{"id", "hash"}).
		AddRow(1, []byte("hash1")).
		AddRow(2, []byte("hash2")).
		AddRow(3, []byte("hash3"))

	mock.ExpectQuery(`SELECT .* FROM "evm_tx_hash_dict" WHERE id IN \(\$1,\$2,\$3\)`).
		WillReturnRows(rows)

	result, err := handler.getHashes(db, txs)

	assert.NoError(t, err)
	assert.Equal(t, 3, len(result)) // Should have 3 unique hashes
	assert.Equal(t, []byte("hash1"), result[1])
	assert.Equal(t, []byte("hash2"), result[2])
	assert.Equal(t, []byte("hash3"), result[3])
	assert.NoError(t, mock.ExpectationsWereMet())
}
