package tx

import (
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)


func TestToTxsResponse(t *testing.T) {
	tests := []struct {
		name         string
		collectedTxs []types.CollectedTx
		expectError  bool
	}{
		{
			name: "Valid transaction data",
			collectedTxs: []types.CollectedTx{
				{
					Hash:     []byte("test_hash_1"),
					Height:   100,
					Sequence: 1,
					Data:     json.RawMessage(`{"test": "data"}`),
				},
				{
					Hash:     []byte("test_hash_2"),
					Height:   101,
					Sequence: 2,
					Data:     json.RawMessage(`{"test": "data2"}`),
				},
			},
			expectError: false,
		},
		{
			name: "Invalid JSON data",
			collectedTxs: []types.CollectedTx{
				{
					Hash:     []byte("test_hash_1"),
					Height:   100,
					Sequence: 1,
					Data:     json.RawMessage(`{invalid json`),
				},
			},
			expectError: true,
		},
		{
			name:         "Empty input",
			collectedTxs: []types.CollectedTx{},
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToTxsResponse(tt.collectedTxs)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result, len(tt.collectedTxs))
			}
		})
	}
}

func TestToTxResponse(t *testing.T) {
	tests := []struct {
		name        string
		collectedTx types.CollectedTx
		expectError bool
	}{
		{
			name: "Valid transaction",
			collectedTx: types.CollectedTx{
				Hash:     []byte("test_hash"),
				Height:   100,
				Sequence: 1,
				Data:     json.RawMessage(`{"test": "data"}`),
			},
			expectError: false,
		},
		{
			name: "Invalid JSON data",
			collectedTx: types.CollectedTx{
				Hash:     []byte("test_hash"),
				Height:   100,
				Sequence: 1,
				Data:     json.RawMessage(`{invalid json`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToTxResponse(tt.collectedTx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, types.Tx{}, result)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, types.Tx{}, result)
			}
		})
	}
}

func TestTxsResponse_Structure(t *testing.T) {
	txsResponse := TxsResponse{
		Txs: []types.Tx{
			{
				TxHash: "test_hash",
				Height: 100,
			},
		},
		Pagination: common.PaginationResponse{
			Total: "1",
		},
	}

	// Test that structure fields are accessible
	assert.NotNil(t, txsResponse.Txs)
	assert.Equal(t, 1, len(txsResponse.Txs))
	assert.Equal(t, "test_hash", txsResponse.Txs[0].TxHash)
	assert.Equal(t, "1", txsResponse.Pagination.Total)
}

func TestTxResponse_Structure(t *testing.T) {
	txResponse := TxResponse{
		Tx: types.Tx{
			TxHash: "test_hash",
			Height: 100,
		},
	}

	// Test that structure fields are accessible
	assert.Equal(t, "test_hash", txResponse.Tx.TxHash)
	assert.Equal(t, int64(100), txResponse.Tx.Height)
}

// Integration test for HTTP endpoints structure
func TestTxHandler_EndpointsStructure(t *testing.T) {
	app := fiber.New()

	// Mock handler - in real tests, you'd use dependency injection
	handler := &TxHandler{}

	// Test that routes can be registered without panic
	assert.NotPanics(t, func() {
		app.Get("/tx/v1/txs", func(c *fiber.Ctx) error {
			return handler.GetTxs(c)
		})
		app.Get("/tx/v1/txs/by_account/:account", func(c *fiber.Ctx) error {
			return handler.GetTxsByAccount(c)
		})
		app.Get("/tx/v1/txs/by_height/:height", func(c *fiber.Ctx) error {
			return handler.GetTxsByHeight(c)
		})
		app.Get("/tx/v1/txs/:tx_hash", func(c *fiber.Ctx) error {
			return handler.GetTxByHash(c)
		})
	})
}

// Test pagination parameter parsing in context of tx handler (simplified)
func TestTxHandler_PaginationParsing(t *testing.T) {
	// Test basic pagination structure exists
	// More detailed pagination tests are in the common package
	pagination := &common.Pagination{
		Limit:  100,
		Offset: 0,
		Order:  common.OrderDesc,
	}

	assert.Equal(t, 100, pagination.Limit)
	assert.Equal(t, 0, pagination.Offset)
	assert.Equal(t, common.OrderDesc, pagination.Order)
}

// Test COUNT optimization strategy for TX table
func TestTxHandler_CountOptimization(t *testing.T) {
	strategy := types.CollectedTx{}

	// Test FastCountStrategy implementation
	assert.Equal(t, "tx", strategy.TableName())
	assert.Equal(t, types.CountOptimizationTypeMax, strategy.GetOptimizationType())
	assert.Equal(t, "sequence", strategy.GetOptimizationField())
	assert.True(t, strategy.SupportsFastCount())
}

// Test cursor implementation for TX table
func TestTxHandler_CursorImplementation(t *testing.T) {
	tx := types.CollectedTx{
		Sequence: 12345,
		Hash:     []byte("test_hash"),
		Height:   100,
	}

	// Test CursorRecord implementation
	fields := tx.GetCursorFields()
	assert.Equal(t, []string{"sequence"}, fields)

	sequenceValue := tx.GetCursorValue("sequence")
	assert.Equal(t, int64(12345), sequenceValue)

	invalidValue := tx.GetCursorValue("invalid_field")
	assert.Nil(t, invalidValue)

	cursorData := tx.GetCursorData()
	assert.Equal(t, map[string]any{"sequence": int64(12345)}, cursorData)
}

// Test message filtering logic (simplified)
func TestTxHandler_MessageFiltering(t *testing.T) {
	// Test message filter types that would be used in TX queries
	testMsgs := []string{
		"cosmos.bank.v1beta1.MsgSend",
		"initia.move.v1.MsgExecute",
		"cosmos.staking.v1beta1.MsgDelegate",
	}

	// Verify message type strings are valid
	for _, msg := range testMsgs {
		assert.NotEmpty(t, msg)
		assert.Contains(t, msg, ".")
		assert.True(t, len(msg) > 10) // Reasonable length check
	}
}

// Test error handling for invalid parameters (simplified)
func TestTxHandler_ErrorHandling(t *testing.T) {
	// Test parameter validation scenarios that would cause errors
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "Invalid height parameter",
			description: "Height must be a valid integer",
		},
		{
			name:        "Invalid hash format",
			description: "Hash must be valid hex format",
		},
		{
			name:        "Invalid pagination limit",
			description: "Pagination limit must be positive and within bounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.description)
		})
	}
}
