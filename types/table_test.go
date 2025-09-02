package types

import (
	"testing"
)

func TestFastCountStrategyImplementation(t *testing.T) {
	tests := []struct {
		name           string
		strategy       FastCountStrategy
		expectedTable  string
		expectedOptType CountOptimizationType
		expectedField  string
		supportsFast   bool
	}{
		{
			name:           "CollectedTx",
			strategy:       CollectedTx{},
			expectedTable:  "tx",
			expectedOptType: CountOptimizationTypeMax,
			expectedField:  "sequence",
			supportsFast:   true,
		},
		{
			name:           "CollectedEvmTx",
			strategy:       CollectedEvmTx{},
			expectedTable:  "evm_tx",
			expectedOptType: CountOptimizationTypeMax,
			expectedField:  "sequence",
			supportsFast:   true,
		},
		{
			name:           "CollectedEvmInternalTx",
			strategy:       CollectedEvmInternalTx{},
			expectedTable:  "evm_internal_tx",
			expectedOptType: CountOptimizationTypeMax,
			expectedField:  "sequence",
			supportsFast:   true,
		},
		{
			name:           "CollectedBlock",
			strategy:       CollectedBlock{},
			expectedTable:  "block",
			expectedOptType: CountOptimizationTypeMax,
			expectedField:  "height",
			supportsFast:   true,
		},
		{
			name:           "CollectedNftCollection",
			strategy:       CollectedNftCollection{},
			expectedTable:  "nft_collection",
			expectedOptType: CountOptimizationTypePgClass,
			expectedField:  "",
			supportsFast:   true,
		},
		{
			name:           "CollectedNft",
			strategy:       CollectedNft{},
			expectedTable:  "nft",
			expectedOptType: CountOptimizationTypePgClass,
			expectedField:  "",
			supportsFast:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategy.TableName(); got != tt.expectedTable {
				t.Errorf("%s.TableName() = %v, want %v", tt.name, got, tt.expectedTable)
			}

			if got := tt.strategy.GetOptimizationType(); got != tt.expectedOptType {
				t.Errorf("%s.GetOptimizationType() = %v, want %v", tt.name, got, tt.expectedOptType)
			}

			if got := tt.strategy.GetOptimizationField(); got != tt.expectedField {
				t.Errorf("%s.GetOptimizationField() = %v, want %v", tt.name, got, tt.expectedField)
			}

			if got := tt.strategy.SupportsFastCount(); got != tt.supportsFast {
				t.Errorf("%s.SupportsFastCount() = %v, want %v", tt.name, got, tt.supportsFast)
			}
		})
	}
}

// CursorRecord interface for testing - copied from common package to avoid circular import
type CursorRecord interface {
	GetCursorFields() []string
	GetCursorValue(field string) any
	GetCursorData() map[string]any
}

func TestCursorRecordImplementation(t *testing.T) {
	tests := []struct {
		name           string
		record         CursorRecord
		expectedFields []string
		testField      string
		expectedValue  any
	}{
		{
			name:           "CollectedTx",
			record:         CollectedTx{Sequence: 12345},
			expectedFields: []string{"sequence"},
			testField:      "sequence",
			expectedValue:  int64(12345),
		},
		{
			name:           "CollectedEvmTx",
			record:         CollectedEvmTx{Sequence: 67890},
			expectedFields: []string{"sequence"},
			testField:      "sequence",
			expectedValue:  int64(67890),
		},
		{
			name:           "CollectedEvmInternalTx",
			record:         CollectedEvmInternalTx{Sequence: 11111},
			expectedFields: []string{"sequence"},
			testField:      "sequence",
			expectedValue:  int64(11111),
		},
		{
			name:           "CollectedBlock",
			record:         CollectedBlock{Height: 100},
			expectedFields: []string{"height"},
			testField:      "height",
			expectedValue:  int64(100),
		},
		{
			name:           "CollectedNftCollection",
			record:         CollectedNftCollection{Height: 200},
			expectedFields: []string{"height"},
			testField:      "height",
			expectedValue:  int64(200),
		},
		{
			name:           "CollectedNft",
			record:         CollectedNft{Height: 300, TokenId: "token123"},
			expectedFields: []string{"height", "token_id"},
			testField:      "height",
			expectedValue:  int64(300),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test GetCursorFields
			fields := tt.record.GetCursorFields()
			if len(fields) != len(tt.expectedFields) {
				t.Errorf("%s.GetCursorFields() length = %d, want %d", tt.name, len(fields), len(tt.expectedFields))
			}
			for i, field := range fields {
				if field != tt.expectedFields[i] {
					t.Errorf("%s.GetCursorFields()[%d] = %s, want %s", tt.name, i, field, tt.expectedFields[i])
				}
			}

			// Test GetCursorValue
			value := tt.record.GetCursorValue(tt.testField)
			if value != tt.expectedValue {
				t.Errorf("%s.GetCursorValue(%s) = %v, want %v", tt.name, tt.testField, value, tt.expectedValue)
			}

			// Test GetCursorValue with invalid field
			invalidValue := tt.record.GetCursorValue("invalid_field")
			if invalidValue != nil {
				t.Errorf("%s.GetCursorValue('invalid_field') = %v, want nil", tt.name, invalidValue)
			}

			// Test GetCursorData
			data := tt.record.GetCursorData()
			if data == nil {
				t.Errorf("%s.GetCursorData() = nil, want map", tt.name)
			}
			if len(data) != len(tt.expectedFields) {
				t.Errorf("%s.GetCursorData() length = %d, want %d", tt.name, len(data), len(tt.expectedFields))
			}
		})
	}
}

func TestCollectedNftCursorComposite(t *testing.T) {
	nft := CollectedNft{
		Height:  500,
		TokenId: "composite_token",
	}

	// Test composite cursor data
	data := nft.GetCursorData()
	expectedData := map[string]any{
		"height":   int64(500),
		"token_id": "composite_token",
	}

	if len(data) != 2 {
		t.Errorf("CollectedNft.GetCursorData() length = %d, want 2", len(data))
	}

	if data["height"] != expectedData["height"] {
		t.Errorf("CollectedNft.GetCursorData()['height'] = %v, want %v", data["height"], expectedData["height"])
	}

	if data["token_id"] != expectedData["token_id"] {
		t.Errorf("CollectedNft.GetCursorData()['token_id'] = %v, want %v", data["token_id"], expectedData["token_id"])
	}

	// Test token_id field specifically
	tokenId := nft.GetCursorValue("token_id")
	if tokenId != "composite_token" {
		t.Errorf("CollectedNft.GetCursorValue('token_id') = %v, want 'composite_token'", tokenId)
	}
}

func TestCountOptimizationTypeConstants(t *testing.T) {
	// Test that constants are properly defined
	if CountOptimizationTypeMax != 1 {
		t.Errorf("CountOptimizationTypeMax = %d, want 1", CountOptimizationTypeMax)
	}

	if CountOptimizationTypePgClass != 2 {
		t.Errorf("CountOptimizationTypePgClass = %d, want 2", CountOptimizationTypePgClass)
	}

	if CountOptimizationTypeCount != 3 {
		t.Errorf("CountOptimizationTypeCount = %d, want 3", CountOptimizationTypeCount)
	}
}

func TestTableNameMethods(t *testing.T) {
	tests := []struct {
		name     string
		table    any
		expected string
	}{
		{"CollectedUpgradeHistory", CollectedUpgradeHistory{}, "upgrade_history"},
		{"CollectedSeqInfo", CollectedSeqInfo{}, "seq_info"},
		{"CollectedBlock", CollectedBlock{}, "block"},
		{"CollectedTx", CollectedTx{}, "tx"},
		{"CollectedEvmTx", CollectedEvmTx{}, "evm_tx"},
		{"CollectedEvmInternalTx", CollectedEvmInternalTx{}, "evm_internal_tx"},
		{"CollectedNftCollection", CollectedNftCollection{}, "nft_collection"},
		{"CollectedNft", CollectedNft{}, "nft"},
		{"CollectedFAStore", CollectedFAStore{}, "fa_store"},
		{"CollectedAccountDict", CollectedAccountDict{}, "account_dict"},
		{"CollectedNftDict", CollectedNftDict{}, "nft_dict"},
		{"CollectedMsgTypeDict", CollectedMsgTypeDict{}, "msg_type_dict"},
		{"CollectedTypeTagDict", CollectedTypeTagDict{}, "type_tag_dict"},
		{"CollectedEvmTxHashDict", CollectedEvmTxHashDict{}, "evm_tx_hash_dict"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use type assertion to call TableName method
			type TableNamer interface {
				TableName() string
			}

			if tn, ok := tt.table.(TableNamer); ok {
				if got := tn.TableName(); got != tt.expected {
					t.Errorf("%s.TableName() = %v, want %v", tt.name, got, tt.expected)
				}
			} else {
				t.Errorf("%s does not implement TableName() method", tt.name)
			}
		})
	}
}