package nft

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
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
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections [get]
func (h *NftHandler) GetCollections(c *fiber.Ctx) error {
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseCollectionQuery()

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var collections []types.CollectedNftCollection
	if err := query.
		Order(pagination.OrderBy("height")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&collections).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	creatorAccounts, err := h.getCollectionCreatorIdMap(collections)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(CollectionsResponse{
		Collections: ToCollectionsResponse(collections, creatorAccounts),
		Pagination:  pagination.ToResponse(total),
	})
}

// GetCollectionsByAccount handles GET /nft/v1/collections/by_account/{account}
// @Summary Get NFT collections by account
// @Description Get NFT collections owned by a specific account
// @Tags NFT
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections/by_account/{account} [get]
func (h *NftHandler) GetCollectionsByAccount(c *fiber.Ctx) error {
	account, err := common.GetAccountParam(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if len(accountIds) == 0 {
		return c.JSON(CollectionsResponse{
			Collections: []Collection{},
			Pagination:  pagination.ToResponse(0),
		})
	}

	query := h.buildBaseCollectionQuery().
		Distinct().
		Joins("INNER JOIN nft ON nft_collection.addr = nft.collection_addr").
		Where("nft.owner_id = ?", accountIds[0])

	var total int64
	if err := h.GetDatabase().Raw(`
		SELECT COUNT(DISTINCT nft_collection.addr) 
		FROM nft_collection 
		INNER JOIN nft ON nft_collection.addr = nft.collection_addr 
		WHERE nft.owner_id = ?`, accountIds[0]).Scan(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var collections []types.CollectedNftCollection
	if err := query.
		Order(pagination.OrderBy("height")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&collections).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	creatorAccounts, err := h.getCollectionCreatorIdMap(collections)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(CollectionsResponse{
		Collections: ToCollectionsResponse(collections, creatorAccounts),
		Pagination:  pagination.ToResponse(total),
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
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} CollectionsResponse
// @Router /indexer/nft/v1/collections/by_name/{name} [get]
func (h *NftHandler) GetCollectionsByName(c *fiber.Ctx) error {
	name, err := common.GetParams(c, "name")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	collections, total := getCollectionByName(h.GetDatabase(), h.GetConfig(), name, pagination)
	creatorAccounts, err := h.getCollectionCreatorIdMap(collections)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(CollectionsResponse{
		Collections: ToCollectionsResponse(collections, creatorAccounts),
		Pagination:  pagination.ToResponse(total),
	})
}

// GetCollectionByCollectionAddr handles GET /nft/v1/collections/{collection_addr}
// @Summary Get NFT Collections By Collection Address
// @Description Get NFT collections for a specific address
// @Tags NFT
// @Accept json
// @Produce json
// @Param collection_addr path string true "Collection address"
// @Success 200 {object} CollectionResponse
// @Router /indexer/nft/v1/collections/{collection_addr} [get]
func (h *NftHandler) GetCollectionByCollectionAddr(c *fiber.Ctx) error {
	collectionAddr, err := common.GetCollectionAddrParam(c, h.GetChainConfig())
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var collection types.CollectedNftCollection
	if err := h.buildBaseCollectionQuery().
		Where("addr = ?", collectionAddr).
		First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "nft collection not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var creatorAccount types.CollectedAccountDict
	if err := h.GetDatabase().
		Where("id = ?", collection.CreatorId).
		First(&creatorAccount).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(CollectionResponse{
		Collection: ToCollectionResponse(collection, creatorAccount.Account),
	})
}

func (h *NftHandler) buildBaseCollectionQuery() *gorm.DB {
	return h.GetDatabase().Model(&types.CollectedNftCollection{})
}
