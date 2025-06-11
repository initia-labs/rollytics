package nft

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

// collections
// GetCollections handles GET /nft/v1/collections
// @Summary Get NFT collections
// @Description Get NFT collections
// @Tags NFT
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections [get]
func (h *NftHandler) GetCollections(c *fiber.Ctx) error {
	req, err := ParseCollectionsRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseCollectionQuery()
	query, err = req.Pagination.ApplyPagination(query, "addr")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}
	var collections []dbtypes.CollectedNftCollection
	if err := query.Find(&collections).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections)
	}

	var nextKey string
	if len(collections) > 0 {
		nextKey = collections[len(collections)-1].Addr
	}

	pageResp, err := req.Pagination.GetPageResponse(len(collections), h.buildBaseCollectionQuery(), nextKey)
	if err != nil {
		return err
	}

	resp := CollectionsResponse{
		Collections: BatchToResponseCollections(collections),
		Pagination:  pageResp,
	}
	return c.JSON(resp)

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
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections/by_account/{account} [get]
func (h *NftHandler) GetCollectionsByAccount(c *fiber.Ctx) error {
	var (
		database = h.GetDatabase()
	)
	req, err := ParseCollectionsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := database.Model(&dbtypes.CollectedNftCollection{}).
		Select("DISTINCT nft_collection.*").
		Joins("INNER JOIN nft ON nft_collection.chain_id = nft.chain_id AND nft_collection.addr = nft.collection_addr").
		Where("nft_collection.chain_id = ?", h.GetChainConfig().ChainId).
		Where("nft.owner = ?", req.Account)

	query, err = req.Pagination.ApplyPagination(query, "height", "addr")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var collections []dbtypes.CollectedNftCollection
	if err := query.Find(&collections).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections+err.Error())
	}

	var nextKey []byte
	if len(collections) > 0 {
		nextKey = common.GetNextKey(collections[len(collections)-1].Height, collections[len(collections)-1].Addr)
	}

	pageResp, err := req.Pagination.GetPageResponse(len(collections), database.Model(&dbtypes.CollectedNft{}).Select("DISTINCT nft.collection_addr").
		Where("chain_id = ? AND owner = ?", h.GetChainConfig().ChainId, req.Account), nextKey)
	if err != nil {
		return err
	}
	resp := CollectionsResponse{
		Collections: BatchToResponseCollections(collections),
		Pagination:  pageResp,
	}
	return c.JSON(resp)
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
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections/by_name/{name} [get]
func (h *NftHandler) GetCollectionsByName(c *fiber.Ctx) error {
	req, err := ParseCollectionsByNameRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseCollectionQuery()
	query = query.Where("name = ?", req.Name)
	query, err = req.Pagination.ApplyPagination(query, "height", "addr")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var collections []dbtypes.CollectedNftCollection
	if err := query.Find(&collections).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections)
	}

	var nextKey []byte
	if len(collections) > 0 {
		nextKey = common.GetNextKey(collections[len(collections)-1].Height, collections[len(collections)-1].Addr)
	}

	pageResp, err := req.Pagination.GetPageResponse(len(collections), h.buildBaseCollectionQuery().Where("name = ?", req.Name), nextKey)
	if err != nil {
		return err
	}
	resp := CollectionsResponse{
		Collections: BatchToResponseCollections(collections),
		Pagination:  pageResp,
	}
	return c.JSON(resp)
}

// GetCollection handles GET /nft/v1/collections/{collection_addr}
// @Summary Get NFT Collections By Collection Address
// @Description Get NFT collections for a specific address
// @Tags NFT
// @Accept json
// @Produce json
// @Param address path string true "Collection address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections/{address} [get]
func (h *NftHandler) GetCollectionByCollection(c *fiber.Ctx) error {
	req, err := ParseCollectionByAddressRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseCollectionQuery()
	query = query.Where("addr = ?", req.CollectionAddr)

	var collection dbtypes.CollectedNftCollection
	if err := query.First(&collection).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fiber.NewError(fiber.StatusNotFound, ErrFailedToFetchCollections)
		}
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchCollections)
	}

	collectionResp := ToResponseCollection(&collection)
	resp := CollectionResponse{
		Collection: collectionResp,
	}

	return c.JSON(resp)
}

func (h *NftHandler) buildBaseCollectionQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedNftCollection{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
