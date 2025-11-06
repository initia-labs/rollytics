package nft

import (
	"database/sql"
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/common-handler/common"
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
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	query := tx.Model(&types.CollectedNftCollection{})

	// Use optimized COUNT - no filters for basic GetCollections
	var strategy types.CollectedNftCollection
	hasFilters := false // no filters in basic collection listing
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters, pagination.CountTotal)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var collections []types.CollectedNftCollection
	finalQuery := pagination.ApplyToNftCollection(query)
	if err := finalQuery.Find(&collections).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	creatorAccounts, err := h.getCollectionCreatorIdMap(tx, collections)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(collections) > 0 {
		lastRecord = collections[len(collections)-1]
	}

	return c.JSON(CollectionsResponse{
		Collections: ToCollectionsResponse(collections, creatorAccounts),
		Pagination:  pagination.ToResponseWithLastRecord(total, len(collections) == pagination.Limit, lastRecord),
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
// @Param pagination.count_total query bool false "Count total, default is true" default is true
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

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if len(accountIds) == 0 {
		return c.JSON(CollectionsResponse{
			Collections: []Collection{},
			Pagination:  pagination.ToResponse(0, false),
		})
	}

	query := tx.Model(&types.CollectedNftCollection{}).
		Distinct().
		Joins("INNER JOIN nft ON nft_collection.addr = nft.collection_addr").
		Where("nft.owner_id = ?", accountIds[0])

	var total int64
	if !pagination.CountTotal {
		total = 0
	} else if err := tx.Raw(`
		SELECT COUNT(DISTINCT nft_collection.addr) 
		FROM nft_collection 
		INNER JOIN nft ON nft_collection.addr = nft.collection_addr 
		WHERE nft.owner_id = ?`, accountIds[0]).Scan(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var collections []types.CollectedNftCollection
	// Complex queries with JOIN have limited cursor application but still attempt to use it
	switch pagination.CursorType {
	case common.CursorTypeHeight:
		if pagination.UseCursor() {
			height := int64(pagination.CursorValue["height"].(float64))
			if pagination.Order == common.OrderDesc {
				query = query.Where("nft_collection.height < ?", height)
			} else {
				query = query.Where("nft_collection.height > ?", height)
			}
			query = query.Order(pagination.OrderBy("nft_collection.height")).Limit(pagination.Limit)
		} else {
			query = query.Order(pagination.OrderBy("nft_collection.height")).Offset(pagination.Offset).Limit(pagination.Limit)
		}
	default:
		query = query.Order(pagination.OrderBy("nft_collection.height")).Offset(pagination.Offset).Limit(pagination.Limit)
	}

	if err := query.Find(&collections).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	creatorAccounts, err := h.getCollectionCreatorIdMap(tx, collections)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(collections) > 0 {
		lastRecord = collections[len(collections)-1]
	}

	return c.JSON(CollectionsResponse{
		Collections: ToCollectionsResponse(collections, creatorAccounts),
		Pagination:  pagination.ToResponseWithLastRecord(total, len(collections) == pagination.Limit, lastRecord),
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
	name, err := common.GetParams(c, "name")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	collections, total := getCollectionByName(h.GetDatabase().DB, h.GetConfig(), name, pagination)
	creatorAccounts, err := h.getCollectionCreatorIdMap(tx, collections)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(collections) > 0 {
		lastRecord = collections[len(collections)-1]
	}

	return c.JSON(CollectionsResponse{
		Collections: ToCollectionsResponse(collections, creatorAccounts),
		Pagination:  pagination.ToResponseWithLastRecord(total, len(collections) == pagination.Limit, lastRecord),
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

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	var collection types.CollectedNftCollection
	if err := tx.Model(&types.CollectedNftCollection{}).
		Where("addr = ?", collectionAddr).
		First(&collection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "nft collection not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var creatorAccount types.CollectedAccountDict
	if err := tx.
		Where("id = ?", collection.CreatorId).
		First(&creatorAccount).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(CollectionResponse{
		Collection: ToCollectionResponse(collection, creatorAccount.Account),
	})
}
