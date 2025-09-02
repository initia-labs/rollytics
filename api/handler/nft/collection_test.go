package nft

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

func TestToCollectionsResponse(t *testing.T) {
	collections := []types.CollectedNftCollection{
		{
			Addr:       []byte("collection_addr_1"),
			Height:     100,
			Name:       "Test Collection 1",
			OriginName: "test_collection_1",
			CreatorId:  1,
			NftCount:   10,
		},
		{
			Addr:       []byte("collection_addr_2"),
			Height:     101,
			Name:       "Test Collection 2",
			OriginName: "test_collection_2",
			CreatorId:  2,
			NftCount:   5,
		},
	}

	creatorAccounts := map[int64][]byte{
		1: []byte("creator_account_1_bytes"),
		2: []byte("creator_account_2_bytes"),
	}

	result := ToCollectionsResponse(collections, creatorAccounts)

	assert.Len(t, result, 2)

	// Test first collection
	assert.NotEmpty(t, result[0].Address)
	assert.Equal(t, int64(100), result[0].Height)
	assert.Equal(t, "Test Collection 1", result[0].CollectionDetail.Name)
	assert.Equal(t, "test_collection_1", result[0].CollectionDetail.OriginName)
	assert.NotEmpty(t, result[0].CollectionDetail.Creator) // Creator is converted to cosmos address format
	assert.Equal(t, int64(10), result[0].CollectionDetail.Nfts.Length)

	// Test second collection
	assert.NotEmpty(t, result[1].Address)
	assert.Equal(t, int64(101), result[1].Height)
	assert.Equal(t, "Test Collection 2", result[1].CollectionDetail.Name)
	assert.Equal(t, "test_collection_2", result[1].CollectionDetail.OriginName)
	assert.NotEmpty(t, result[1].CollectionDetail.Creator) // Creator is converted to cosmos address format
	assert.Equal(t, int64(5), result[1].CollectionDetail.Nfts.Length)
}

func TestToCollectionResponse(t *testing.T) {
	collection := types.CollectedNftCollection{
		Addr:       []byte("collection_addr"),
		Height:     200,
		Name:       "Single Test Collection",
		OriginName: "single_test_collection",
		CreatorId:  100,
		NftCount:   25,
	}

	creatorAccount := []byte("single_creator_account_bytes")

	result := ToCollectionResponse(collection, creatorAccount)

	assert.NotEmpty(t, result.Address)
	assert.Equal(t, int64(200), result.Height)
	assert.Equal(t, "Single Test Collection", result.CollectionDetail.Name)
	assert.Equal(t, "single_test_collection", result.CollectionDetail.OriginName)
	assert.NotEmpty(t, result.CollectionDetail.Creator) // Creator is converted to cosmos address format
	assert.Equal(t, int64(25), result.CollectionDetail.Nfts.Length)
}

func TestCollectionsResponse_Structure(t *testing.T) {
	collectionsResponse := CollectionsResponse{
		Collections: []Collection{
			{
				Address: "test_addr",
				Height:  100,
				CollectionDetail: CollectionDetail{
					Name:    "Test Collection",
					Creator: "test_creator",
				},
			},
		},
		Pagination: common.PaginationResponse{
			Total: "1",
		},
	}

	// Test structure accessibility
	assert.NotNil(t, collectionsResponse.Collections)
	assert.Equal(t, 1, len(collectionsResponse.Collections))
	assert.Equal(t, "test_addr", collectionsResponse.Collections[0].Address)
	assert.Equal(t, "1", collectionsResponse.Pagination.Total)
}

func TestCollectionResponse_Structure(t *testing.T) {
	collectionResponse := CollectionResponse{
		Collection: Collection{
			Address: "single_addr",
			Height:  200,
			CollectionDetail: CollectionDetail{
				Name:    "Single Collection",
				Creator: "single_creator",
			},
		},
	}

	// Test structure accessibility
	assert.Equal(t, "single_addr", collectionResponse.Collection.Address)
	assert.Equal(t, int64(200), collectionResponse.Collection.Height)
	assert.Equal(t, "Single Collection", collectionResponse.Collection.CollectionDetail.Name)
	assert.Equal(t, "single_creator", collectionResponse.Collection.CollectionDetail.Creator)
}

// Test COUNT optimization strategy for NFT Collection table
func TestNftCollectionHandler_CountOptimization(t *testing.T) {
	strategy := types.CollectedNftCollection{}

	// Test FastCountStrategy implementation
	assert.Equal(t, "nft_collection", strategy.TableName())
	assert.Equal(t, types.CountOptimizationTypePgClass, strategy.GetOptimizationType())
	assert.Equal(t, "", strategy.GetOptimizationField()) // pg_class doesn't use field
	assert.True(t, strategy.SupportsFastCount())
}

// Test cursor implementation for NFT Collection table
func TestNftCollectionHandler_CursorImplementation(t *testing.T) {
	collection := types.CollectedNftCollection{
		Height:   500,
		Addr:     []byte("test_collection_addr"),
		Name:     "Test Collection",
		NftCount: 100,
	}

	// Test CursorRecord implementation
	fields := collection.GetCursorFields()
	assert.Equal(t, []string{"height"}, fields)

	heightValue := collection.GetCursorValue("height")
	assert.Equal(t, int64(500), heightValue)

	invalidValue := collection.GetCursorValue("invalid_field")
	assert.Nil(t, invalidValue)

	cursorData := collection.GetCursorData()
	assert.Equal(t, map[string]any{"height": int64(500)}, cursorData)
}

func TestToCollectionsResponse_EmptyInput(t *testing.T) {
	collections := []types.CollectedNftCollection{}
	creatorAccounts := map[int64][]byte{}

	result := ToCollectionsResponse(collections, creatorAccounts)

	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestToCollectionsResponse_MissingCreatorAccount(t *testing.T) {
	collections := []types.CollectedNftCollection{
		{
			Addr:       []byte("collection_addr"),
			Height:     100,
			Name:       "Test Collection",
			OriginName: "test_collection",
			CreatorId:  999, // Not in creatorAccounts map
			NftCount:   10,
		},
	}

	creatorAccounts := map[int64][]byte{
		1: []byte("existing_creator_bytes"),
	}

	result := ToCollectionsResponse(collections, creatorAccounts)

	assert.Len(t, result, 1)
	// Missing creator ID should result in empty byte slice, converted to empty address
}

// Test collection name handling
func TestCollection_Names(t *testing.T) {
	collection := types.CollectedNftCollection{
		Name:       "Display Name",
		OriginName: "origin_name",
	}

	result := ToCollectionResponse(collection, []byte("test_creator_bytes"))

	assert.Equal(t, "Display Name", result.CollectionDetail.Name)
	assert.Equal(t, "origin_name", result.CollectionDetail.OriginName)
}

// Test collection address handling (should convert bytes to hex)
func TestCollection_AddressHandling(t *testing.T) {
	addrBytes := []byte{0x01, 0x02, 0x03, 0x04}
	collection := types.CollectedNftCollection{
		Addr: addrBytes,
	}

	result := ToCollectionResponse(collection, []byte("test_creator_bytes"))

	// The address should be converted to hex representation
	assert.NotEmpty(t, result.Address)
	assert.NotEqual(t, string(addrBytes), result.Address) // Should be hex, not raw bytes
}

// Test NFT count accuracy
func TestCollection_NftCount(t *testing.T) {
	tests := []struct {
		name     string
		nftCount int64
		expected int64
	}{
		{"Zero NFTs", 0, 0},
		{"Positive NFTs", 100, 100},
		{"Large number", 999999, 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection := types.CollectedNftCollection{
				NftCount: tt.nftCount,
			}

			result := ToCollectionResponse(collection, []byte("test_creator_bytes"))
			assert.Equal(t, tt.expected, result.CollectionDetail.Nfts.Length)
		})
	}
}
