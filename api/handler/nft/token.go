package nft

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
)

// GetTokensByOwner handles GET /nft/v1/tokens/by_account/{account}
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
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_account/{account} [get]
func (h *NftHandler) GetTokensByOwner(c *fiber.Ctx) error {
	req, err := ParseTokensByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseNftQuery().
		Where("owner = ?", req.Account)
	if req.CollectionAddr != "" {
		query = query.Where("collection_addr = ?", req.CollectionAddr)
	}
	if req.TokenId != "" {
		query = query.Where("token_id = ?", req.TokenId)
	}
	query, err = req.Pagination.Apply(query, "collection_addr", "token_id")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var nfts []dbtypes.CollectedNft
	if err := query.Find(&nfts).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// response
	pageResp := common.GetPageResponse(req.Pagination, nfts, func(nft dbtypes.CollectedNft) []any {
		return []any{nft.CollectionAddr, nft.TokenId}
	}, func() int64 {
		var total int64
		if err := h.GetDatabase().Model(&dbtypes.CollectedNft{}).
			Where("chain_id = ? AND owner = ?", h.GetChainConfig().ChainId, req.Account).
			Count(&total).Error; err != nil {
			return 0
		}
		return total
	})

	tokens, err := BatchToResponseNfts(h.GetDatabase(), nfts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(NftsResponse{
		Tokens:     tokens,
		Pagination: pageResp,
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
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_collection/{collection_addr} [get]
func (h *NftHandler) GetTokensByCollectionAddr(c *fiber.Ctx) error {
	req, err := ParseTokensByCollectionRequest(h.GetChainConfig(), c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseNftQuery().
		Where("collection_addr = ?", req.CollectionAddr)

	if req.TokenId != "" {
		query = query.Where("token_id = ?", req.TokenId)
	}

	query, err = req.Pagination.Apply(query, "token_id")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var nfts []dbtypes.CollectedNft
	if err := query.Find(&nfts).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// get pagination response
	pageResp := common.GetPageResponse(req.Pagination, nfts, func(token dbtypes.CollectedNft) []any {
		return []any{token.TokenId}
	}, func() int64 {
		var total int64
		if err := h.GetDatabase().Model(&dbtypes.CollectedNft{}).
			Where("chain_id = ? AND collection_addr = ?", h.GetChainConfig().ChainId, req.CollectionAddr).
			Count(&total).Error; err != nil {
			return 0
		}
		return total
	})
	tokens, err := BatchToResponseNfts(h.GetDatabase(), nfts)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(NftsResponse{
		Tokens:     tokens,
		Pagination: pageResp,
	})
}

func (h *NftHandler) buildBaseNftQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedNft{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
