package common

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/initia-labs/rollytics/types"
)

func TestDetectCursorType(t *testing.T) {
	tests := []struct {
		name         string
		cursorData   map[string]any
		expectedType CursorType
	}{
		{
			name: "Sequence cursor",
			cursorData: map[string]any{
				"sequence": 12345,
			},
			expectedType: CursorTypeSequence,
		},
		{
			name: "Height cursor",
			cursorData: map[string]any{
				"height": 100,
			},
			expectedType: CursorTypeHeight,
		},
		{
			name: "Composite cursor",
			cursorData: map[string]any{
				"height":   100,
				"token_id": "test",
			},
			expectedType: CursorTypeComposite,
		},
		{
			name:         "Empty cursor",
			cursorData:   map[string]any{},
			expectedType: CursorTypeOffset,
		},
		{
			name: "Unknown field",
			cursorData: map[string]any{
				"unknown": "value",
			},
			expectedType: CursorTypeOffset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectCursorType(tt.cursorData)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestCursorTypeString(t *testing.T) {
	tests := []struct {
		cursorType CursorType
		expected   string
	}{
		{CursorTypeOffset, "offset"},
		{CursorTypeSequence, "sequence"},
		{CursorTypeHeight, "height"},
		{CursorTypeComposite, "composite"},
		{CursorType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.cursorType.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPagination_UseCursor(t *testing.T) {
	tests := []struct {
		name       string
		cursorType CursorType
		expected   bool
	}{
		{"Offset type", CursorTypeOffset, false},
		{"Zero type", CursorType(0), false},
		{"Sequence type", CursorTypeSequence, true},
		{"Height type", CursorTypeHeight, true},
		{"Composite type", CursorTypeComposite, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pagination{CursorType: tt.cursorType}
			result := p.UseCursor()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPagination_OrderBy(t *testing.T) {
	tests := []struct {
		name     string
		order    string
		keys     []string
		expected string
	}{
		{
			name:     "Single key DESC",
			order:    OrderDesc,
			keys:     []string{"sequence"},
			expected: "sequence DESC",
		},
		{
			name:     "Single key ASC",
			order:    OrderAsc,
			keys:     []string{"height"},
			expected: "height ASC",
		},
		{
			name:     "Multiple keys",
			order:    OrderDesc,
			keys:     []string{"height", "token_id"},
			expected: "height DESC, token_id DESC",
		},
		{
			name:     "No keys",
			order:    OrderDesc,
			keys:     []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pagination{Order: tt.order}
			result := p.OrderBy(tt.keys...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPagination_ToResponse(t *testing.T) {
	p := &Pagination{
		Limit:  100,
		Offset: 200,
	}

	// Test with data that should have next page
	result := p.ToResponse(500) // total > offset + limit

	assert.Equal(t, "500", result.Total)
	assert.NotNil(t, result.NextKey, "Should have next key when more data exists")
	assert.NotNil(t, result.PreviousKey, "Should have previous key when offset > limit")

	// Test without next page
	result2 := p.ToResponse(250) // total <= offset + limit

	assert.Equal(t, "250", result2.Total)
	assert.Nil(t, result2.NextKey, "Should not have next key when no more data")
}

func TestBase64CursorEncoding(t *testing.T) {
	// Test JSON cursor encoding/decoding
	originalData := map[string]any{
		"sequence": float64(12345),
		"height":   float64(100),
	}

	// Encode
	jsonBytes, err := json.Marshal(originalData)
	assert.NoError(t, err)

	encoded := base64.StdEncoding.EncodeToString(jsonBytes)
	assert.NotEmpty(t, encoded)

	// Decode
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	assert.NoError(t, err)

	var decodedData map[string]any
	err = json.Unmarshal(decoded, &decodedData)
	assert.NoError(t, err)
	assert.Equal(t, originalData, decodedData)
}

func TestPaginationConstants(t *testing.T) {
	// Test that constants are properly defined
	assert.Equal(t, 100, DefaultLimit)
	assert.Equal(t, 1000, MaxLimit)
	assert.Equal(t, 0, DefaultOffset)
	assert.Equal(t, "DESC", OrderDesc)
	assert.Equal(t, "ASC", OrderAsc)
}

func TestCursorTypeConstants(t *testing.T) {
	// Test that cursor type constants are distinct
	assert.NotEqual(t, CursorTypeOffset, CursorTypeSequence)
	assert.NotEqual(t, CursorTypeSequence, CursorTypeHeight)
	assert.NotEqual(t, CursorTypeHeight, CursorTypeComposite)

	// Test that they have expected values
	assert.Equal(t, CursorType(1), CursorTypeOffset)
	assert.Equal(t, CursorType(2), CursorTypeSequence)
	assert.Equal(t, CursorType(3), CursorTypeHeight)
	assert.Equal(t, CursorType(4), CursorTypeComposite)
}

// MockCursorRecord for testing ToResponseWithLastRecord
type MockCursorRecord struct {
	data map[string]any
}

func (m MockCursorRecord) GetCursorFields() []string {
	fields := make([]string, 0, len(m.data))
	for k := range m.data {
		fields = append(fields, k)
	}
	return fields
}

func (m MockCursorRecord) GetCursorValue(field string) any {
	return m.data[field]
}

func (m MockCursorRecord) GetCursorData() map[string]any {
	return m.data
}

func TestPagination_ToResponseWithLastRecord(t *testing.T) {
	tests := []struct {
		name         string
		pagination   *Pagination
		lastRecord   any
		expectedNext bool
	}{
		{
			name: "With cursor record",
			pagination: &Pagination{
				CursorType: CursorTypeSequence,
			},
			lastRecord: MockCursorRecord{
				data: map[string]any{"sequence": int64(12345)},
			},
			expectedNext: true,
		},
		{
			name: "Offset pagination",
			pagination: &Pagination{
				CursorType: CursorTypeOffset,
				Limit:      10,
				Offset:     0,
			},
			lastRecord:   nil,
			expectedNext: false,
		},
		{
			name:         "Nil record",
			pagination:   &Pagination{CursorType: CursorTypeSequence},
			lastRecord:   nil,
			expectedNext: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pagination.ToResponseWithLastRecord(100, tt.lastRecord)

			assert.Equal(t, "100", result.Total)

			if tt.expectedNext && tt.lastRecord != nil {
				if _, ok := tt.lastRecord.(CursorRecord); ok {
					assert.NotNil(t, result.NextKey)
				}
			}
		})
	}
}

// Test CursorRecord interface implementation
type TestCursorRecord struct {
	sequence int64
	height   int64
	tokenId  string
}

func (t TestCursorRecord) GetCursorFields() []string {
	if t.tokenId != "" {
		return []string{"height", "token_id"}
	}
	if t.height != 0 {
		return []string{"height"}
	}
	return []string{"sequence"}
}

func (t TestCursorRecord) GetCursorValue(field string) any {
	switch field {
	case "sequence":
		return t.sequence
	case "height":
		return t.height
	case "token_id":
		return t.tokenId
	default:
		return nil
	}
}

func (t TestCursorRecord) GetCursorData() map[string]any {
	data := make(map[string]any)
	if t.sequence != 0 {
		data["sequence"] = t.sequence
	}
	if t.height != 0 {
		data["height"] = t.height
	}
	if t.tokenId != "" {
		data["token_id"] = t.tokenId
	}
	return data
}

func TestCursorRecordInterface(t *testing.T) {
	tests := []struct {
		name   string
		record CursorRecord
	}{
		{
			name: "Sequence record",
			record: TestCursorRecord{
				sequence: 12345,
			},
		},
		{
			name: "Height record",
			record: TestCursorRecord{
				height: 100,
			},
		},
		{
			name: "Composite record",
			record: TestCursorRecord{
				height:  200,
				tokenId: "test_token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := tt.record.GetCursorFields()
			assert.NotEmpty(t, fields, "Should have cursor fields")

			data := tt.record.GetCursorData()
			assert.NotEmpty(t, data, "Should have cursor data")

			// Test that all fields can be retrieved
			for _, field := range fields {
				value := tt.record.GetCursorValue(field)
				assert.NotNil(t, value, "Field %s should have a value", field)
				assert.Equal(t, data[field], value, "GetCursorValue should match GetCursorData")
			}
		})
	}
}

// Test all table types from types/table.go with their table names
func TestAllTableNames(t *testing.T) {
	tests := []struct {
		name         string
		table        interface{ TableName() string }
		expectedName string
	}{
		{"CollectedUpgradeHistory", types.CollectedUpgradeHistory{}, "upgrade_history"},
		{"CollectedSeqInfo", types.CollectedSeqInfo{}, "seq_info"},
		{"CollectedBlock", types.CollectedBlock{}, "block"},
		{"CollectedTx", types.CollectedTx{}, "tx"},
		{"CollectedEvmTx", types.CollectedEvmTx{}, "evm_tx"},
		{"CollectedEvmInternalTx", types.CollectedEvmInternalTx{}, "evm_internal_tx"},
		{"CollectedNftCollection", types.CollectedNftCollection{}, "nft_collection"},
		{"CollectedNft", types.CollectedNft{}, "nft"},
		{"CollectedFAStore", types.CollectedFAStore{}, "fa_store"},
		{"CollectedAccountDict", types.CollectedAccountDict{}, "account_dict"},
		{"CollectedNftDict", types.CollectedNftDict{}, "nft_dict"},
		{"CollectedMsgTypeDict", types.CollectedMsgTypeDict{}, "msg_type_dict"},
		{"CollectedTypeTagDict", types.CollectedTypeTagDict{}, "type_tag_dict"},
		{"CollectedEvmTxHashDict", types.CollectedEvmTxHashDict{}, "evm_tx_hash_dict"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedName, tt.table.TableName())
		})
	}
}

// Test all FastCountStrategy implementations from types/table.go
func TestAllFastCountStrategies(t *testing.T) {
	tests := []struct {
		name                string
		strategy            types.FastCountStrategy
		expectedTable       string
		expectedOptType     types.CountOptimizationType
		expectedField       string
		expectedSupports    bool
	}{
		{
			name:                "CollectedTx MAX optimization",
			strategy:            types.CollectedTx{},
			expectedTable:       "tx",
			expectedOptType:     types.CountOptimizationTypeMax,
			expectedField:       "sequence",
			expectedSupports:    true,
		},
		{
			name:                "CollectedEvmTx MAX optimization",
			strategy:            types.CollectedEvmTx{},
			expectedTable:       "evm_tx",
			expectedOptType:     types.CountOptimizationTypeMax,
			expectedField:       "sequence",
			expectedSupports:    true,
		},
		{
			name:                "CollectedEvmInternalTx MAX optimization",
			strategy:            types.CollectedEvmInternalTx{},
			expectedTable:       "evm_internal_tx",
			expectedOptType:     types.CountOptimizationTypeMax,
			expectedField:       "sequence",
			expectedSupports:    true,
		},
		{
			name:                "CollectedBlock MAX optimization",
			strategy:            types.CollectedBlock{},
			expectedTable:       "block",
			expectedOptType:     types.CountOptimizationTypeMax,
			expectedField:       "height",
			expectedSupports:    true,
		},
		{
			name:                "CollectedNftCollection pg_class optimization",
			strategy:            types.CollectedNftCollection{},
			expectedTable:       "nft_collection",
			expectedOptType:     types.CountOptimizationTypePgClass,
			expectedField:       "",
			expectedSupports:    true,
		},
		{
			name:                "CollectedNft pg_class optimization",
			strategy:            types.CollectedNft{},
			expectedTable:       "nft",
			expectedOptType:     types.CountOptimizationTypePgClass,
			expectedField:       "",
			expectedSupports:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedTable, tt.strategy.TableName())
			assert.Equal(t, tt.expectedOptType, tt.strategy.GetOptimizationType())
			assert.Equal(t, tt.expectedField, tt.strategy.GetOptimizationField())
			assert.Equal(t, tt.expectedSupports, tt.strategy.SupportsFastCount())
		})
	}
}

// Test all CursorRecord implementations from types/table.go
func TestAllCursorRecordImplementations(t *testing.T) {
	tests := []struct {
		name              string
		record            CursorRecord
		expectedFields    []string
		testFieldValue    map[string]any
	}{
		{
			name:           "CollectedTx sequence cursor",
			record:         types.CollectedTx{Sequence: 12345},
			expectedFields: []string{"sequence"},
			testFieldValue: map[string]any{"sequence": int64(12345)},
		},
		{
			name:           "CollectedEvmTx sequence cursor",
			record:         types.CollectedEvmTx{Sequence: 67890},
			expectedFields: []string{"sequence"},
			testFieldValue: map[string]any{"sequence": int64(67890)},
		},
		{
			name:           "CollectedEvmInternalTx sequence cursor",
			record:         types.CollectedEvmInternalTx{Sequence: 11111},
			expectedFields: []string{"sequence"},
			testFieldValue: map[string]any{"sequence": int64(11111)},
		},
		{
			name:           "CollectedBlock height cursor",
			record:         types.CollectedBlock{Height: 100},
			expectedFields: []string{"height"},
			testFieldValue: map[string]any{"height": int64(100)},
		},
		{
			name:           "CollectedNftCollection height cursor",
			record:         types.CollectedNftCollection{Height: 200},
			expectedFields: []string{"height"},
			testFieldValue: map[string]any{"height": int64(200)},
		},
		{
			name:           "CollectedNft composite cursor",
			record:         types.CollectedNft{Height: 300, TokenId: "token123"},
			expectedFields: []string{"height", "token_id"},
			testFieldValue: map[string]any{"height": int64(300), "token_id": "token123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test cursor fields
			fields := tt.record.GetCursorFields()
			assert.Equal(t, tt.expectedFields, fields)

			// Test cursor data
			cursorData := tt.record.GetCursorData()
			assert.Equal(t, tt.testFieldValue, cursorData)

			// Test individual field values
			for field, expectedValue := range tt.testFieldValue {
				value := tt.record.GetCursorValue(field)
				assert.Equal(t, expectedValue, value)
			}

			// Test invalid field returns nil
			invalidValue := tt.record.GetCursorValue("invalid_field")
			assert.Nil(t, invalidValue)
		})
	}
}

// Test cursor type detection with all table types
func TestCursorTypeDetectionAllTables(t *testing.T) {
	tests := []struct {
		name         string
		record       CursorRecord
		expectedType CursorType
	}{
		{
			name:         "CollectedTx produces sequence cursor",
			record:       types.CollectedTx{Sequence: 1},
			expectedType: CursorTypeSequence,
		},
		{
			name:         "CollectedEvmTx produces sequence cursor",
			record:       types.CollectedEvmTx{Sequence: 1},
			expectedType: CursorTypeSequence,
		},
		{
			name:         "CollectedEvmInternalTx produces sequence cursor",
			record:       types.CollectedEvmInternalTx{Sequence: 1},
			expectedType: CursorTypeSequence,
		},
		{
			name:         "CollectedBlock produces height cursor",
			record:       types.CollectedBlock{Height: 1},
			expectedType: CursorTypeHeight,
		},
		{
			name:         "CollectedNftCollection produces height cursor",
			record:       types.CollectedNftCollection{Height: 1},
			expectedType: CursorTypeHeight,
		},
		{
			name:         "CollectedNft produces composite cursor",
			record:       types.CollectedNft{Height: 1, TokenId: "test"},
			expectedType: CursorTypeComposite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursorData := tt.record.GetCursorData()
			detectedType := detectCursorType(cursorData)
			assert.Equal(t, tt.expectedType, detectedType)
		})
	}
}

// Test pagination with all cursor types from actual table implementations
func TestPaginationWithAllTableCursors(t *testing.T) {
	tests := []struct {
		name       string
		record     CursorRecord
		pagination *Pagination
	}{
		{
			name:   "TX table sequence-based pagination",
			record: types.CollectedTx{Sequence: 12345},
			pagination: &Pagination{
				CursorType: CursorTypeSequence,
				Order:      OrderDesc,
				Limit:      100,
			},
		},
		{
			name:   "Block table height-based pagination",
			record: types.CollectedBlock{Height: 100},
			pagination: &Pagination{
				CursorType: CursorTypeHeight,
				Order:      OrderDesc,
				Limit:      50,
			},
		},
		{
			name:   "NFT table composite pagination",
			record: types.CollectedNft{Height: 200, TokenId: "token_abc"},
			pagination: &Pagination{
				CursorType: CursorTypeComposite,
				Order:      OrderAsc,
				Limit:      25,
			},
		},
		{
			name:   "EVM TX table sequence-based pagination",
			record: types.CollectedEvmTx{Sequence: 54321},
			pagination: &Pagination{
				CursorType: CursorTypeSequence,
				Order:      OrderDesc,
				Limit:      75,
			},
		},
		{
			name:   "EVM Internal TX table sequence-based pagination",
			record: types.CollectedEvmInternalTx{Sequence: 98765},
			pagination: &Pagination{
				CursorType: CursorTypeSequence,
				Order:      OrderAsc,
				Limit:      200,
			},
		},
		{
			name:   "NFT Collection table height-based pagination",
			record: types.CollectedNftCollection{Height: 150},
			pagination: &Pagination{
				CursorType: CursorTypeHeight,
				Order:      OrderDesc,
				Limit:      30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that pagination uses cursor correctly
			assert.True(t, tt.pagination.UseCursor())

			// Test response generation with cursor record
			response := tt.pagination.ToResponseWithLastRecord(1000, tt.record)
			assert.Equal(t, "1000", response.Total)
			assert.NotNil(t, response.NextKey, "Should generate next key from cursor record")

			// Verify cursor fields are available
			fields := tt.record.GetCursorFields()
			assert.NotEmpty(t, fields, "Cursor record should have fields")

			// Test ordering with cursor fields
			orderBy := tt.pagination.OrderBy(fields...)
			assert.NotEmpty(t, orderBy, "Should generate ORDER BY clause")
		})
	}
}

// Test COUNT optimization categorization by table type
func TestCountOptimizationByTableType(t *testing.T) {
	maxOptimizationTables := []types.FastCountStrategy{
		types.CollectedTx{},
		types.CollectedEvmTx{},
		types.CollectedEvmInternalTx{},
		types.CollectedBlock{},
	}

	pgClassOptimizationTables := []types.FastCountStrategy{
		types.CollectedNftCollection{},
		types.CollectedNft{},
	}

	// Test MAX optimization tables
	for i, strategy := range maxOptimizationTables {
		t.Run(fmt.Sprintf("MAX optimization table %d", i), func(t *testing.T) {
			assert.Equal(t, types.CountOptimizationTypeMax, strategy.GetOptimizationType())
			assert.NotEmpty(t, strategy.GetOptimizationField(), "MAX optimization should have a field")
			assert.True(t, strategy.SupportsFastCount())
		})
	}

	// Test pg_class optimization tables
	for i, strategy := range pgClassOptimizationTables {
		t.Run(fmt.Sprintf("pg_class optimization table %d", i), func(t *testing.T) {
			assert.Equal(t, types.CountOptimizationTypePgClass, strategy.GetOptimizationType())
			assert.Empty(t, strategy.GetOptimizationField(), "pg_class optimization should not have a field")
			assert.True(t, strategy.SupportsFastCount())
		})
	}
}

// Test table name consistency across all types
func TestTableNameConsistencyAllTypes(t *testing.T) {
	expectedNames := map[string]string{
		"CollectedUpgradeHistory": "upgrade_history",
		"CollectedSeqInfo":        "seq_info",
		"CollectedBlock":          "block",
		"CollectedTx":             "tx",
		"CollectedEvmTx":          "evm_tx",
		"CollectedEvmInternalTx":  "evm_internal_tx",
		"CollectedNftCollection":  "nft_collection",
		"CollectedNft":            "nft",
		"CollectedFAStore":        "fa_store",
		"CollectedAccountDict":    "account_dict",
		"CollectedNftDict":        "nft_dict",
		"CollectedMsgTypeDict":    "msg_type_dict",
		"CollectedTypeTagDict":    "type_tag_dict",
		"CollectedEvmTxHashDict":  "evm_tx_hash_dict",
	}

	allTables := map[string]interface{ TableName() string }{
		"CollectedUpgradeHistory": types.CollectedUpgradeHistory{},
		"CollectedSeqInfo":        types.CollectedSeqInfo{},
		"CollectedBlock":          types.CollectedBlock{},
		"CollectedTx":             types.CollectedTx{},
		"CollectedEvmTx":          types.CollectedEvmTx{},
		"CollectedEvmInternalTx":  types.CollectedEvmInternalTx{},
		"CollectedNftCollection":  types.CollectedNftCollection{},
		"CollectedNft":            types.CollectedNft{},
		"CollectedFAStore":        types.CollectedFAStore{},
		"CollectedAccountDict":    types.CollectedAccountDict{},
		"CollectedNftDict":        types.CollectedNftDict{},
		"CollectedMsgTypeDict":    types.CollectedMsgTypeDict{},
		"CollectedTypeTagDict":    types.CollectedTypeTagDict{},
		"CollectedEvmTxHashDict":  types.CollectedEvmTxHashDict{},
	}

	for typeName, table := range allTables {
		t.Run(typeName, func(t *testing.T) {
			actualName := table.TableName()
			expectedName := expectedNames[typeName]
			assert.Equal(t, expectedName, actualName)
			assert.NotEmpty(t, actualName, "Table name should not be empty")
		})
	}
}

// Test comprehensive cursor field mapping for all cursor-enabled tables
func TestAllTableCursorFieldMapping(t *testing.T) {
	cursorFieldMappings := map[string]struct {
		record         CursorRecord
		expectedFields []string
		optimizationField string
	}{
		"TX Tables": {
			record: types.CollectedTx{},
			expectedFields: []string{"sequence"},
			optimizationField: "sequence",
		},
		"EVM TX Tables": {
			record: types.CollectedEvmTx{},
			expectedFields: []string{"sequence"},
			optimizationField: "sequence",
		},
		"EVM Internal TX Tables": {
			record: types.CollectedEvmInternalTx{},
			expectedFields: []string{"sequence"},
			optimizationField: "sequence",
		},
		"Block Tables": {
			record: types.CollectedBlock{},
			expectedFields: []string{"height"},
			optimizationField: "height",
		},
		"NFT Collection Tables": {
			record: types.CollectedNftCollection{},
			expectedFields: []string{"height"},
			optimizationField: "", // pg_class doesn't use field
		},
		"NFT Tables": {
			record: types.CollectedNft{},
			expectedFields: []string{"height", "token_id"},
			optimizationField: "", // pg_class doesn't use field
		},
	}

	for tableName, mapping := range cursorFieldMappings {
		t.Run(tableName, func(t *testing.T) {
			// Test cursor fields
			cursorFields := mapping.record.GetCursorFields()
			assert.Equal(t, mapping.expectedFields, cursorFields)

			// Test optimization field (if FastCountStrategy)
			if strategy, ok := mapping.record.(types.FastCountStrategy); ok {
				optimizationField := strategy.GetOptimizationField()
				assert.Equal(t, mapping.optimizationField, optimizationField)
			}
		})
	}
}

// Test all table type constants and enums
func TestAllTableTypeConstants(t *testing.T) {
	// Test CountOptimizationType constants
	assert.Equal(t, types.CountOptimizationType(1), types.CountOptimizationTypeMax)
	assert.Equal(t, types.CountOptimizationType(2), types.CountOptimizationTypePgClass)
	assert.Equal(t, types.CountOptimizationType(3), types.CountOptimizationTypeCount)

	// Test CursorType constants
	assert.Equal(t, CursorType(1), CursorTypeOffset)
	assert.Equal(t, CursorType(2), CursorTypeSequence)
	assert.Equal(t, CursorType(3), CursorTypeHeight)
	assert.Equal(t, CursorType(4), CursorTypeComposite)

	// Test all constants are distinct
	allCountOptTypes := []types.CountOptimizationType{
		types.CountOptimizationTypeMax,
		types.CountOptimizationTypePgClass,
		types.CountOptimizationTypeCount,
	}
	for i := 0; i < len(allCountOptTypes); i++ {
		for j := i + 1; j < len(allCountOptTypes); j++ {
			assert.NotEqual(t, allCountOptTypes[i], allCountOptTypes[j])
		}
	}

	allCursorTypes := []CursorType{
		CursorTypeOffset,
		CursorTypeSequence,
		CursorTypeHeight,
		CursorTypeComposite,
	}
	for i := 0; i < len(allCursorTypes); i++ {
		for j := i + 1; j < len(allCursorTypes); j++ {
			assert.NotEqual(t, allCursorTypes[i], allCursorTypes[j])
		}
	}
}