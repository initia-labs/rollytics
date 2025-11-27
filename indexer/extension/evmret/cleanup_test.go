package evmret

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create tables
	err = db.AutoMigrate(&types.CollectedTx{}, &types.CollectedAccountDict{}, &types.CollectedTxAccount{})
	require.NoError(t, err)

	return db
}

func TestIsValidEVMAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{
			name:     "valid EVM address",
			address:  "0x6ed1637781269560b204c27Cd42d95e057C4BE44",
			expected: true,
		},
		{
			name:     "valid lowercase EVM address",
			address:  "0x6ed1637781269560b204c27cd42d95e057c4be44",
			expected: true,
		},
		{
			name:     "empty 0x",
			address:  "0x",
			expected: false,
		},
		{
			name:     "too short",
			address:  "0x123",
			expected: false,
		},
		{
			name:     "too long",
			address:  "0x6ed1637781269560b204c27Cd42d95e057C4BE4412",
			expected: false,
		},
		{
			name:     "no 0x prefix",
			address:  "6ed1637781269560b204c27Cd42d95e057C4BE44",
			expected: false,
		},
		{
			name:     "invalid hex characters",
			address:  "0x6ed1637781269560b204c27Cd42d95e057C4BEZZ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEVMAddress(tt.address)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAddressesFromValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []string
	}{
		{
			name:     "single valid address",
			value:    "0x6ed1637781269560b204c27Cd42d95e057C4BE44",
			expected: []string{"0x6ed1637781269560b204c27cd42d95e057c4be44"},
		},
		{
			name:     "multiple comma-separated addresses",
			value:    "0x6ed1637781269560b204c27Cd42d95e057C4BE44,0xABCDEF1234567890123456789012345678901234",
			expected: []string{"0x6ed1637781269560b204c27cd42d95e057c4be44", "0xabcdef1234567890123456789012345678901234"},
		},
		{
			name:     "empty 0x",
			value:    "0x",
			expected: []string{"0x0000000000000000000000000000000000000000"},
		},
		{
			name:     "mixed valid and invalid",
			value:    "0x6ed1637781269560b204c27Cd42d95e057C4BE44,0x123,invalid",
			expected: []string{"0x6ed1637781269560b204c27cd42d95e057c4be44", "0x0000000000000000000000000000000000000123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAddressesFromValue(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindRetOnlyAddresses(t *testing.T) {
	tests := []struct {
		name     string
		txData   string
		expected []string
		wantErr  bool
	}{
		{
			name: "ret-only address",
			txData: `{
				"events": [
					{
						"type": "call",
						"attributes": [
							{"key": "ret", "value": "0x6ed1637781269560b204c27cd42d95e057c4be44"}
						]
					}
				]
			}`,
			expected: []string{"0x6ed1637781269560b204c27cd42d95e057c4be44"},
			wantErr:  false,
		},
		{
			name: "address in both ret and non-ret",
			txData: `{
				"events": [
					{
						"type": "call",
						"attributes": [
							{"key": "contract", "value": "0x6ed1637781269560b204c27cd42d95e057c4be44"},
							{"key": "ret", "value": "0x6ed1637781269560b204c27cd42d95e057c4be44"}
						]
					}
				]
			}`,
			expected: []string{},
			wantErr:  false,
		},
		{
			name: "multiple addresses, one ret-only",
			txData: `{
				"events": [
					{
						"type": "call",
						"attributes": [
							{"key": "contract", "value": "0x1111111111111111111111111111111111111111"},
							{"key": "ret", "value": "0x2222222222222222222222222222222222222222"},
							{"key": "ret", "value": "0x1111111111111111111111111111111111111111"}
						]
					}
				]
			}`,
			expected: []string{"0x2222222222222222222222222222222222222222"},
			wantErr:  false,
		},
		{
			name: "empty ret value",
			txData: `{
				"events": [
					{
						"type": "call",
						"attributes": [
							{"key": "ret", "value": "0x"}
						]
					}
				]
			}`,
			expected: []string{"0x0000000000000000000000000000000000000000"},
			wantErr:  false,
		},
		{
			name:     "invalid JSON",
			txData:   `{invalid json}`,
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindRetOnlyAddresses(json.RawMessage(tt.txData))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

func TestFilterNonSigners(t *testing.T) {
	tests := []struct {
		name       string
		accountIds []int64
		signerId   int64
		expected   []int64
	}{
		{
			name:       "filter out signer",
			accountIds: []int64{1, 2, 3},
			signerId:   1,
			expected:   []int64{2, 3},
		},
		{
			name:       "all non-signers",
			accountIds: []int64{2, 3},
			signerId:   1,
			expected:   []int64{2, 3},
		},
		{
			name:       "all signers",
			accountIds: []int64{1},
			signerId:   1,
			expected:   []int64{},
		},
		{
			name:       "different sequence",
			accountIds: []int64{4},
			signerId:   4,
			expected:   []int64{},
		},
		{
			name:       "empty list",
			accountIds: []int64{},
			signerId:   1,
			expected:   []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FilterNonSigners(tt.accountIds, tt.signerId)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestDeleteRetOnlyRecords(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Insert test tx_accounts records
	txAccounts := []types.CollectedTxAccount{
		{AccountId: 1, Sequence: 100, Signer: false},
		{AccountId: 2, Sequence: 100, Signer: false},
		{AccountId: 3, Sequence: 100, Signer: true},
		{AccountId: 1, Sequence: 200, Signer: false},
	}
	for _, ta := range txAccounts {
		require.NoError(t, db.Create(&ta).Error)
	}

	tests := []struct {
		name       string
		accountIds []int64
		sequence   int64
		expected   int64
	}{
		{
			name:       "delete specific records",
			accountIds: []int64{1, 2},
			sequence:   100,
			expected:   2,
		},
		{
			name:       "no matching records",
			accountIds: []int64{999},
			sequence:   100,
			expected:   0,
		},
		{
			name:       "empty list",
			accountIds: []int64{},
			sequence:   100,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleted, err := DeleteRetOnlyRecords(ctx, db, tt.accountIds, tt.sequence)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, deleted)
		})
	}

	// Verify that records were actually deleted
	var count int64
	db.Model(&types.CollectedTxAccount{}).Where("sequence = ? AND account_id IN ?", 100, []int64{1, 2}).Count(&count)
	assert.Equal(t, int64(0), count, "Records should be deleted")

	// Verify that other records still exist
	db.Model(&types.CollectedTxAccount{}).Where("sequence = ? AND account_id = ?", 100, 3).Count(&count)
	assert.Equal(t, int64(1), count, "Signer record should not be deleted")

	db.Model(&types.CollectedTxAccount{}).Where("sequence = ? AND account_id = ?", 200, 1).Count(&count)
	assert.Equal(t, int64(1), count, "Different sequence record should not be deleted")
}

func TestProcessBatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Ensure caches are initialized for any dictionary lookups inside processing
	util.InitializeCaches(&config.CacheConfig{
		AccountCacheSize:          1024,
		NftCacheSize:              1024,
		MsgTypeCacheSize:          256,
		TypeTagCacheSize:          256,
		EvmTxHashCacheSize:        1024,
		EvmDenomContractCacheSize: 256,
	})

	// Insert test data (Account field is []byte in types.CollectedAccountDict)
	accounts := []types.CollectedAccountDict{
		{Id: 1, Account: hexToBytes("1111111111111111111111111111111111111111")},
		{Id: 2, Account: hexToBytes("2222222222222222222222222222222222222222")},
	}
	for _, acc := range accounts {
		require.NoError(t, db.Create(&acc).Error)
	}

	txData := `{
		"events": [
			{
				"type": "call",
				"attributes": [
					{"key": "ret", "value": "0x1111111111111111111111111111111111111111"}
				]
			}
		]
	}`

	// Hash field is []byte in types.CollectedTx
	txs := []types.CollectedTx{
		{Hash: []byte("hash1"), Height: 100, Sequence: 1, Data: json.RawMessage(txData)},
		{Hash: []byte("hash2"), Height: 101, Sequence: 2, Data: json.RawMessage(txData)},
	}
	for _, tx := range txs {
		require.NoError(t, db.Create(&tx).Error)
	}

	txAccounts := []types.CollectedTxAccount{
		{AccountId: 1, Sequence: 1, Signer: false},
		{AccountId: 1, Sequence: 2, Signer: false},
	}
	for _, ta := range txAccounts {
		require.NoError(t, db.Create(&ta).Error)
	}

	// Create a test logger
	logger := newTestLogger()

	// Process batch
	deleted, err := ProcessBatch(ctx, db, logger, 100, 101)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	// Verify records were deleted
	var count int64
	db.Model(&types.CollectedTxAccount{}).Count(&count)
	assert.Equal(t, int64(0), count, "All ret-only records should be deleted")
}

func TestProcessBatchWithContextCancellation(t *testing.T) {
	db := setupTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())

	// Insert many test transactions (Hash field is []byte in types.CollectedTx)
	for i := 0; i < 100; i++ {
		tx := types.CollectedTx{
			Hash:     []byte{byte(i)},
			Height:   int64(i),
			Sequence: int64(i),
			Data:     json.RawMessage(`{"events": []}`),
		}
		require.NoError(t, db.Create(&tx).Error)
	}

	// Cancel context immediately
	cancel()

	logger := newTestLogger()
	_, err := ProcessBatch(ctx, db, logger, 0, 99)
	assert.Error(t, err)
	// Error should contain context.Canceled, may be wrapped
	assert.ErrorIs(t, err, context.Canceled)
}

// Helper function to create a test logger
func newTestLogger() *slog.Logger {
	return slog.Default()
}

// hexToBytes converts a hex string to bytes
func hexToBytes(hexStr string) []byte {
	b := make([]byte, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		b[i/2] = hexCharToByte(hexStr[i])<<4 | hexCharToByte(hexStr[i+1])
	}
	return b
}

// hexCharToByte converts a single hex character to its byte value
func hexCharToByte(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// isValidEVMAddress checks if a string is a valid EVM address
func isValidEVMAddress(addr string) bool {
	// Must start with 0x
	if !strings.HasPrefix(addr, "0x") {
		return false
	}

	// Remove 0x prefix
	hexPart := addr[2:]

	// Must be exactly 40 hex characters (20 bytes)
	if len(hexPart) != 40 {
		return false
	}

	// Must be valid hex
	_, err := hex.DecodeString(hexPart)
	return err == nil
}
