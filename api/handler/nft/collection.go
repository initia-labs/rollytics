package nft

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
)

// GetCollections handles GET /nft/v1/collections
// @Summary Get NFT collections
// @Description Get NFT collections
// @Tags NFT
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections [get]
func (h *NftHandler) GetCollections(c *fiber.Ctx) error {
	req, err := ParseCollectionsRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	collections, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedNftCollection](req.Pagination).
		WithQuery(h.buildBaseCollectionQuery()).
		WithKeys("addr").
		WithKeyExtractor(func(col dbtypes.CollectedNftCollection) []any {
			return []any{col.Addr}
		}).
		Execute()

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections)
	}

	return c.JSON(CollectionsResponse{
		Collections: BatchToResponseCollections(collections),
		Pagination:  pageResp,
	})
}

// GetCollectionsByAccount handles GET /nft/v1/collections/by_account/{account}
// @Summary Get NFT collections by owner account
// @Description Get NFT collections owned by a specific account
// @Tags NFT
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections/by_account/{account} [get]
func (h *NftHandler) GetCollectionsByAccount(c *fiber.Ctx) error {
	req, err := ParseCollectionsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	chainId := h.GetChainConfig().ChainId

	query := h.GetDatabase().Model(&dbtypes.CollectedNftCollection{}).
		Select("DISTINCT nft_collection.*").
		Joins("INNER JOIN nft ON nft_collection.chain_id = nft.chain_id AND nft_collection.addr = nft.collection_addr").
		Where("nft_collection.chain_id = ?", chainId).
		Where("nft.owner = ?", req.Account)

	countQuery := h.GetDatabase().Model(&dbtypes.CollectedNft{}).
		Select("DISTINCT nft.collection_addr").
		Where("chain_id = ? AND owner = ?", chainId, req.Account)

	collections, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedNftCollection](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("nft_collection.height", "nft_collection.addr").
		WithKeyExtractor(func(col dbtypes.CollectedNftCollection) []any {
			return []any{col.Height, col.Addr}
		}).
		Execute()

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections)
	}

	return c.JSON(CollectionsResponse{
		Collections: BatchToResponseCollections(collections),
		Pagination:  pageResp,
	})
}

// GetCollectionsByName handles GET /nft/v1/collections/by_name/{name}
// @Summary Get NFT collections by name
// @Description Get NFT collections for a specific name
// @Tags NFT
// @Accept json
// @Produce json
// @Param name path string true "Collection name"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections/by_name/{name} [get]
func (h *NftHandler) GetCollectionsByName(c *fiber.Ctx) error {
	req, err := ParseCollectionsByNameRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseCollectionQuery().
		Where("name = ?", req.Name)

	countQuery := h.buildBaseCollectionQuery().
		Where("name = ?", req.Name)

	collections, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedNftCollection](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("height", "addr").
		WithKeyExtractor(func(col dbtypes.CollectedNftCollection) []any {
			return []any{col.Height, col.Addr}
		}).
		Execute()

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections)
	}

	return c.JSON(CollectionsResponse{
		Collections: BatchToResponseCollections(collections),
		Pagination:  pageResp,
	})
}

// GetCollectionByCollection handles GET /nft/v1/collections/{collection_addr}
// @Summary Get NFT Collections By Collection Address
// @Description Get NFT collections for a specific address
// @Tags NFT
// @Accept json
// @Produce json
// @Param collection_addr path string true "Collection address"
// @Success 200 {object} CollectionResponse
// @Router /indexer/nft/v1/collections/{collection_addr} [get]
func (h *NftHandler) GetCollectionByCollection(c *fiber.Ctx) error {
	req, err := ParseCollectionByAddressRequest(h.GetChainConfig(), c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var collection dbtypes.CollectedNftCollection
	if err := h.buildBaseCollectionQuery().
		Where("addr = ?", req.CollectionAddr).
		First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "NFT collection not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections)
	}

	return c.JSON(CollectionResponse{
		Collection: ToResponseCollection(&collection),
	})
}

func (h *NftHandler) buildBaseCollectionQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedNftCollection{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
