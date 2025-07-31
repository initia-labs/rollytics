package nft

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

// GetTokensByAccount handles GET /nft/v1/tokens/by_account/{account}
// @Summary Get NFT tokens by account
// @Description Get NFT tokens owned by a specific account
// @Tags NFT
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param collection_addr query string false "Collection address to filter by (optional)"
// @Param token_id query string false "Token ID to filter by (optional)"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_account/{account} [get]
func (h *NftHandler) GetTokensByAccount(c *fiber.Ctx) error {
	account, err := common.GetAccountParam(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	collectionAddr, err := common.GetCollectionAddrQuery(c, h.GetChainConfig())
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	tokenId := c.Query("token_id")
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Get account ID from account_dict
	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	query := h.buildBaseNftQuery().Where("owner_id = ?", accountIds[0])

	if collectionAddr != "" {
		query = query.Where("collection_addr = ?", collectionAddr)
	}
	if tokenId != "" {
		query = query.Where("token_id = ?", tokenId)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var nfts []types.CollectedNft
	if err := query.
		Order(pagination.OrderBy("collection_addr", "token_id")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&nfts).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	ownerAccounts, err := h.getNftOwnerIdMap(nfts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	nftsRes, err := ToNftsResponse(h.GetDatabase(), nfts, ownerAccounts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(NftsResponse{
		Tokens:     nftsRes,
		Pagination: pagination.ToResponse(total),
	})
}

// GetTokensByCollectionAddr handles GET /nft/v1/tokens/by_collection/{collection_addr}
// @Summary Get NFT tokens by collection
// @Description Get NFT tokens from a specific collection
// @Tags NFT
// @Accept json
// @Produce json
// @Param collection_addr path string true "Collection address"
// @Param token_id query string false "Token ID to filter by (optional)"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_collection/{collection_addr} [get]
func (h *NftHandler) GetTokensByCollectionAddr(c *fiber.Ctx) error {
	collectionAddr, err := common.GetCollectionAddrParam(c, h.GetChainConfig())
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	tokenId := c.Query("token_id")
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseNftQuery().Where("collection_addr = ?", collectionAddr)

	if tokenId != "" {
		query = query.Where("token_id = ?", tokenId)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var nfts []types.CollectedNft
	if err := query.
		Order(pagination.OrderBy("token_id")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&nfts).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	ownerAccounts, err := h.getNftOwnerIdMap(nfts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	nftsRes, err := ToNftsResponse(h.GetDatabase(), nfts, ownerAccounts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(NftsResponse{
		Tokens:     nftsRes,
		Pagination: pagination.ToResponse(total),
	})
}

func (h *NftHandler) buildBaseNftQuery() *gorm.DB {
	return h.GetDatabase().Model(&types.CollectedNft{})
}
