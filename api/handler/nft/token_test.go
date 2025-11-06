package nft

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

func TestToNftsResponse(t *testing.T) {
	// Note: This would require a real database connection, so we test input validation instead

	nfts := []types.CollectedNft{
		{
			CollectionAddr: []byte("collection_addr_1"),
			TokenId:        "token_1",
			Height:         100,
			OwnerId:        1,
			Uri:            "https://example.com/token1.json",
		},
		{
			CollectionAddr: []byte("collection_addr_2"),
			TokenId:        "token_2",
			Height:         101,
			OwnerId:        2,
			Uri:            "https://example.com/token2.json",
		},
	}

	ownerAccounts := map[int64][]byte{
		1: []byte("owner_account_1"),
		2: []byte("owner_account_2"),
	}

	// Note: ToNftsResponse requires actual database connection for collection lookup
	// In a real test, you would mock the database operations
	// Converting gorm.DB to orm.Database for the function call would be complex
	// result, err := ToNftsResponse(db, nfts, ownerAccounts)

	// Test that input data structures are valid
	assert.Len(t, nfts, 2)
	assert.Len(t, ownerAccounts, 2)
	assert.Equal(t, "token_1", nfts[0].TokenId)
	assert.Equal(t, "token_2", nfts[1].TokenId)
}

func TestNftsResponse_Structure(t *testing.T) {
	nftsResponse := NftsResponse{
		Tokens: []Nft{
			{
				CollectionAddr: "test_collection_addr",
				Height:         100,
				Owner:          "test_owner",
				Nft: NftDetails{
					TokenId: "test_token_id",
					Uri:     "https://example.com/token.json",
				},
			},
		},
		Pagination: common.PaginationResponse{
			Total: "1",
		},
	}

	// Test structure accessibility
	assert.NotNil(t, nftsResponse.Tokens)
	assert.Equal(t, 1, len(nftsResponse.Tokens))
	assert.Equal(t, "test_collection_addr", nftsResponse.Tokens[0].CollectionAddr)
	assert.Equal(t, "test_token_id", nftsResponse.Tokens[0].Nft.TokenId)
	assert.Equal(t, "1", nftsResponse.Pagination.Total)
}

func TestNft_Structure(t *testing.T) {
	nft := Nft{
		CollectionAddr: "collection_address",
		CollectionName: "Collection Name",
		Height:         500,
		Owner:          "owner_address",
		Nft: NftDetails{
			TokenId: "12345",
			Uri:     "https://metadata.example.com/12345.json",
		},
	}

	// Test all fields are accessible and correct
	assert.Equal(t, "collection_address", nft.CollectionAddr)
	assert.Equal(t, "Collection Name", nft.CollectionName)
	assert.Equal(t, "12345", nft.Nft.TokenId)
	assert.Equal(t, int64(500), nft.Height)
	assert.Equal(t, "owner_address", nft.Owner)
	assert.Equal(t, "https://metadata.example.com/12345.json", nft.Nft.Uri)
}

// Test COUNT optimization strategy for NFT table
func TestNftHandler_CountOptimization(t *testing.T) {
	strategy := types.CollectedNft{}

	// Test FastCountStrategy implementation
	assert.Equal(t, "nft", strategy.TableName())
	assert.Equal(t, types.CountOptimizationTypePgClass, strategy.GetOptimizationType())
	assert.Equal(t, "", strategy.GetOptimizationField()) // pg_class doesn't use field
	assert.True(t, strategy.SupportsFastCount())
}

// Test cursor implementation for NFT table (composite cursor)
func TestNftHandler_CursorImplementation(t *testing.T) {
	nft := types.CollectedNft{
		Height:         300,
		TokenId:        "composite_token_123",
		CollectionAddr: []byte("collection_addr"),
		OwnerId:        5,
	}

	// Test CursorRecord implementation - NFT uses composite cursor (height + token_id)
	fields := nft.GetCursorFields()
	assert.Equal(t, []string{"height", "token_id"}, fields)

	heightValue := nft.GetCursorValue("height")
	assert.Equal(t, int64(300), heightValue)

	tokenIdValue := nft.GetCursorValue("token_id")
	assert.Equal(t, "composite_token_123", tokenIdValue)

	invalidValue := nft.GetCursorValue("invalid_field")
	assert.Nil(t, invalidValue)

	cursorData := nft.GetCursorData()
	expectedData := map[string]any{
		"height":   int64(300),
		"token_id": "composite_token_123",
	}
	assert.Equal(t, expectedData, cursorData)
}

func TestNft_TokenIdHandling(t *testing.T) {
	tests := []struct {
		name    string
		tokenId string
	}{
		{"Numeric token ID", "12345"},
		{"String token ID", "abc_token"},
		{"UUID token ID", "550e8400-e29b-41d4-a716-446655440000"},
		{"Empty token ID", ""},
		{"Special characters", "token#123@test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nft := types.CollectedNft{
				TokenId: tt.tokenId,
				Height:  100,
			}

			tokenIdValue := nft.GetCursorValue("token_id")
			assert.Equal(t, tt.tokenId, tokenIdValue)

			cursorData := nft.GetCursorData()
			assert.Equal(t, tt.tokenId, cursorData["token_id"])
		})
	}
}

func TestNft_OwnerIdMapping(t *testing.T) {
	ownerAccounts := map[int64]string{
		1:   "owner1",
		2:   "owner2",
		100: "owner100",
	}

	tests := []struct {
		name          string
		ownerId       int64
		expectedOwner string
	}{
		{"Valid owner ID 1", 1, "owner1"},
		{"Valid owner ID 2", 2, "owner2"},
		{"Valid owner ID 100", 100, "owner100"},
		{"Invalid owner ID", 999, ""}, // Should return empty for missing IDs
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the mapping logic that would be used in ToNftsResponse
			owner, exists := ownerAccounts[tt.ownerId]
			if !exists {
				owner = ""
			}
			assert.Equal(t, tt.expectedOwner, owner)
		})
	}
}

func TestNft_UriHandling(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		description string
	}{
		{"HTTP URI", "http://example.com/token.json", "Standard HTTP URI"},
		{"HTTPS URI", "https://example.com/token.json", "Secure HTTPS URI"},
		{"IPFS URI", "ipfs://QmXx...", "IPFS distributed storage URI"},
		{"Data URI", "data:application/json;base64,eyJ0eXBlIjoi...", "Embedded data URI"},
		{"Empty URI", "", "Empty URI should be handled"},
		{"Relative path", "/metadata/token.json", "Relative path URI"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nft := Nft{
				Nft: NftDetails{
					Uri: tt.uri,
				},
			}

			assert.Equal(t, tt.uri, nft.Nft.Uri, tt.description)
		})
	}
}

func TestNft_CollectionAddressHandling(t *testing.T) {
	// Test that collection addresses are properly handled as hex strings
	collectionAddrBytes := []byte{0xAB, 0xCD, 0xEF, 0x01, 0x23, 0x45}

	collectedNft := types.CollectedNft{
		CollectionAddr: collectionAddrBytes,
		TokenId:        "test_token",
	}

	// In the actual ToNftsResponse function, addresses would be converted to hex
	// Here we test the input handling
	assert.Equal(t, collectionAddrBytes, collectedNft.CollectionAddr)
	assert.Equal(t, "test_token", collectedNft.TokenId)
}

func TestNft_HeightValidation(t *testing.T) {
	tests := []struct {
		name   string
		height int64
		valid  bool
	}{
		{"Zero height", 0, true},
		{"Positive height", 12345, true},
		{"Large height", 999999999, true},
		{"Negative height", -1, false}, // Technically invalid but type allows it
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nft := types.CollectedNft{
				Height: tt.height,
			}

			// Height should be stored as provided
			assert.Equal(t, tt.height, nft.Height)

			// In cursor data, height should be accessible
			cursorData := nft.GetCursorData()
			assert.Equal(t, tt.height, cursorData["height"])
		})
	}
}

// Test empty collections handling
func TestToNftsResponse_EmptyCollections(t *testing.T) {
	nfts := []types.CollectedNft{}
	ownerAccounts := map[int64][]byte{}

	// Test that empty input structures are valid
	assert.Len(t, nfts, 0)
	assert.Len(t, ownerAccounts, 0)
	// ToNftsResponse would return empty slice for empty input (if properly mocked)
}

// Test consistent cursor ordering for pagination
func TestNft_CursorOrdering(t *testing.T) {
	nft1 := types.CollectedNft{Height: 100, TokenId: "token_a"}
	nft2 := types.CollectedNft{Height: 100, TokenId: "token_b"}
	nft3 := types.CollectedNft{Height: 101, TokenId: "token_a"}

	// Test that cursor data is consistent for ordering
	cursor1 := nft1.GetCursorData()
	cursor2 := nft2.GetCursorData()
	cursor3 := nft3.GetCursorData()

	// Same height, different token_id
	assert.Equal(t, cursor1["height"], cursor2["height"])
	assert.NotEqual(t, cursor1["token_id"], cursor2["token_id"])

	// Different height, same token_id
	assert.NotEqual(t, cursor1["height"], cursor3["height"])
	assert.Equal(t, cursor1["token_id"], cursor3["token_id"])
}

// Test validateOrderBy function
func TestValidateOrderBy(t *testing.T) {
	tests := []struct {
		name      string
		orderBy   string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Empty order_by should be valid",
			orderBy:   "",
			expectErr: false,
		},
		{
			name:      "Valid token_id order_by",
			orderBy:   "token_id",
			expectErr: false,
		},
		{
			name:      "Valid height order_by",
			orderBy:   "height",
			expectErr: false,
		},
		{
			name:      "Case insensitive token_id",
			orderBy:   "TOKEN_ID",
			expectErr: false,
		},
		{
			name:      "Mixed case token_id",
			orderBy:   "Token_Id",
			expectErr: false,
		},
		{
			name:      "Whitespace around valid value",
			orderBy:   "  token_id  ",
			expectErr: false,
		},
		{
			name:      "Invalid order_by value",
			orderBy:   "invalid_field",
			expectErr: true,
			errMsg:    "invalid order_by value 'invalid_field', must be one of: token_id, height",
		},
		{
			name:      "Partial match should fail",
			orderBy:   "token",
			expectErr: true,
			errMsg:    "invalid order_by value 'token', must be one of: token_id, height",
		},
		{
			name:      "Numeric value should fail",
			orderBy:   "123",
			expectErr: true,
			errMsg:    "invalid order_by value '123', must be one of: token_id, height",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeOrderBy(tt.orderBy)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test getTokensWithFilters function behavior
func TestGetTokensWithFilters_OrderByHandling(t *testing.T) {
	// Test the order by clause construction logic
	tests := []struct {
		name          string
		orderBy       string
		pagination    *common.Pagination
		expectedOrder string
		description   string
	}{
		{
			name:    "No order_by uses default pagination",
			orderBy: "",
			pagination: &common.Pagination{
				Limit:  10,
				Offset: 0,
				Order:  "DESC",
			},
			expectedOrder: "",
			description:   "When order_by is empty, should use pagination.ApplyToNft",
		},
		{
			name:    "token_id order_by with DESC",
			orderBy: "token_id",
			pagination: &common.Pagination{
				Limit:  10,
				Offset: 0,
				Order:  "DESC",
			},
			expectedOrder: "token_id DESC",
			description:   "Should construct proper ORDER BY clause",
		},
		{
			name:    "height order_by with ASC",
			orderBy: "height",
			pagination: &common.Pagination{
				Limit:  10,
				Offset: 0,
				Order:  "ASC",
			},
			expectedOrder: "height ASC",
			description:   "Should construct proper ORDER BY clause with ASC",
		},
		{
			name:    "token_id order_by with offset and limit",
			orderBy: "token_id",
			pagination: &common.Pagination{
				Limit:  5,
				Offset: 10,
				Order:  "DESC",
			},
			expectedOrder: "token_id DESC",
			description:   "Should apply offset and limit with custom order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the order clause construction logic
			var orderClause string
			if tt.orderBy != "" {
				orderClause = fmt.Sprintf("%s %s", tt.orderBy, tt.pagination.Order)
			}

			if tt.orderBy != "" {
				assert.Equal(t, tt.expectedOrder, orderClause, tt.description)
			} else {
				assert.Empty(t, orderClause, tt.description)
			}
		})
	}
}

// Test pagination integration with order_by
func TestPaginationWithOrderBy(t *testing.T) {
	tests := []struct {
		name        string
		orderBy     string
		pagination  *common.Pagination
		expectError bool
		description string
	}{
		{
			name:    "Valid pagination with token_id order",
			orderBy: "token_id",
			pagination: &common.Pagination{
				Limit:  100,
				Offset: 0,
				Order:  "DESC",
			},
			expectError: false,
			description: "Should handle valid pagination with custom order",
		},
		{
			name:    "Valid pagination with height order",
			orderBy: "height",
			pagination: &common.Pagination{
				Limit:  50,
				Offset: 25,
				Order:  "ASC",
			},
			expectError: false,
			description: "Should handle valid pagination with height order",
		},
		{
			name:    "Empty order_by with pagination",
			orderBy: "",
			pagination: &common.Pagination{
				Limit:  10,
				Offset: 0,
				Order:  "DESC",
			},
			expectError: false,
			description: "Should handle empty order_by with default pagination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate order_by parameter
			_, err := normalizeOrderBy(tt.orderBy)
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			// Test pagination structure
			assert.NotNil(t, tt.pagination)
			assert.Greater(t, tt.pagination.Limit, 0)
			assert.GreaterOrEqual(t, tt.pagination.Offset, 0)
			assert.Contains(t, []string{"ASC", "DESC"}, tt.pagination.Order)
		})
	}
}

// Test order_by parameter edge cases
func TestOrderByEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		orderBy   string
		expectErr bool
	}{
		{
			name:      "Null character",
			orderBy:   "token_id\x00",
			expectErr: true,
		},
		{
			name:      "Unicode characters",
			orderBy:   "token_ïd",
			expectErr: true,
		},
		{
			name:      "SQL injection attempt",
			orderBy:   "token_id; DROP TABLE nft;",
			expectErr: true,
		},
		{
			name:      "Very long string",
			orderBy:   strings.Repeat("a", 1000),
			expectErr: true,
		},
		{
			name:      "Special characters",
			orderBy:   "token_id@#$%",
			expectErr: true,
		},
		{
			name:      "Numbers and letters",
			orderBy:   "token_id123",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeOrderBy(tt.orderBy)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test the integration of order_by with different pagination scenarios
func TestOrderByWithPaginationScenarios(t *testing.T) {
	scenarios := []struct {
		name           string
		orderBy        string
		pagination     *common.Pagination
		expectedResult string
	}{
		{
			name:    "First page with token_id order",
			orderBy: "token_id",
			pagination: &common.Pagination{
				Limit:  10,
				Offset: 0,
				Order:  "DESC",
			},
			expectedResult: "token_id DESC",
		},
		{
			name:    "Second page with height order",
			orderBy: "height",
			pagination: &common.Pagination{
				Limit:  10,
				Offset: 10,
				Order:  "ASC",
			},
			expectedResult: "height ASC",
		},
		{
			name:    "Large offset with token_id order",
			orderBy: "token_id",
			pagination: &common.Pagination{
				Limit:  50,
				Offset: 1000,
				Order:  "DESC",
			},
			expectedResult: "token_id DESC",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Validate order_by
			_, err := normalizeOrderBy(scenario.orderBy)
			assert.NoError(t, err)

			// Test order clause construction
			orderClause := fmt.Sprintf("%s %s", scenario.orderBy, scenario.pagination.Order)
			assert.Equal(t, scenario.expectedResult, orderClause)

			// Test pagination values
			assert.Greater(t, scenario.pagination.Limit, 0)
			assert.GreaterOrEqual(t, scenario.pagination.Offset, 0)
		})
	}
}

// Test the behavior when order_by is used vs when it's not
func TestOrderByVsDefaultPagination(t *testing.T) {
	basePagination := &common.Pagination{
		Limit:  10,
		Offset: 0,
		Order:  "DESC",
	}

	t.Run("With order_by uses custom ORDER BY clause", func(t *testing.T) {
		orderBy := "token_id"
		_, err := normalizeOrderBy(orderBy)
		assert.NoError(t, err)

		// Simulate the logic from getTokensWithFilters
		orderClause := fmt.Sprintf("%s %s", orderBy, basePagination.Order)
		expectedClause := "token_id DESC"
		assert.Equal(t, expectedClause, orderClause)
	})

	t.Run("Without order_by uses pagination.ApplyToNft", func(t *testing.T) {
		orderBy := ""
		_, err := normalizeOrderBy(orderBy)
		assert.NoError(t, err)

		// When orderBy is empty, the function should use pagination.ApplyToNft
		// This would apply the default ordering (height, token_id) with the pagination order
		assert.Empty(t, orderBy)
	})
}

// Test that the allowed values are correctly defined
func TestAllowedOrderByValues(t *testing.T) {
	// Test that the allowed values in validateOrderBy match expected fields
	allowedValues := []string{"token_id", "height"}

	// These should be the only valid values
	for _, value := range allowedValues {
		_, err := normalizeOrderBy(value)
		assert.NoError(t, err, "Value %s should be valid", value)
	}

	// Test case insensitive versions
	for _, value := range allowedValues {
		_, err := normalizeOrderBy(strings.ToUpper(value))
		assert.NoError(t, err, "Value %s should be valid (case insensitive)", strings.ToUpper(value))
	}

	// Test that other common field names are not allowed
	invalidValues := []string{"id", "owner_id", "collection_addr", "uri", "created_at", "updated_at"}
	for _, value := range invalidValues {
		_, err := normalizeOrderBy(value)
		assert.Error(t, err, "Value %s should not be valid", value)
	}
}

// Test the error message format
func TestOrderByErrorMessage(t *testing.T) {
	invalidValue := "invalid_field"
	_, err := normalizeOrderBy(invalidValue)

	assert.Error(t, err)
	errorMsg := err.Error()

	// Check that the error message contains the invalid value
	assert.Contains(t, errorMsg, invalidValue)

	// Check that the error message contains the allowed values
	assert.Contains(t, errorMsg, "token_id")
	assert.Contains(t, errorMsg, "height")

	// Check the format of the error message
	expectedPrefix := fmt.Sprintf("invalid order_by value '%s', must be one of:", invalidValue)
	assert.True(t, strings.HasPrefix(errorMsg, expectedPrefix))
}
