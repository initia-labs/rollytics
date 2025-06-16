package nft

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

// GetTokensByAccount handles GET /nft/v1/tokens/by_account/{account}
// @Summary Get NFT tokens by account
// @Description Get NFT tokens owned by a specific account
// @Tags NFT
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_account/{account} [get]
func (h *NftHandler) GetTokensByAccount(c *fiber.Ctx) error {
	req, err := ParseTokensByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseNftQuery().
		Where("owner = ?", req.Account)

	countQuery := h.buildBaseNftQuery().
		Where("owner = ?", req.Account)

	tokens, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedNft](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("collection_addr", "token_id").
		WithKeyExtractor(func(nft dbtypes.CollectedNft) []any {
			return []any{nft.CollectionAddr, nft.TokenId}
		}).
		Execute()

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNft)
	}

	return c.JSON(NftsResponse{
		Tokens:     BatchToResponseNfts(tokens),
		Pagination: pageResp,
	})
}

// GetTokensByCollection handles GET /nft/v1/tokens/by_collection/{collection_addr}
// @Summary Get NFT tokens by collection
// @Description Get NFT tokens from a specific collection
// @Tags NFT
// @Accept json
// @Produce json
// @Param collection_addr path string true "Collection address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_collection/{collection_addr} [get]
func (h *NftHandler) GetTokensByCollection(c *fiber.Ctx) error {
	req, err := ParseTokensByCollectionRequest(h.GetChainConfig(), c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseNftQuery().
		Where("collection_addr = ?", req.CollectionAddr)

	countQuery := h.buildBaseNftQuery().
		Where("collection_addr = ?", req.CollectionAddr)

	tokens, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedNft](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("token_id").
		WithKeyExtractor(func(nft dbtypes.CollectedNft) []any {
			return []any{nft.TokenId}
		}).Execute()

	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNft)
	}

	return c.JSON(NftsResponse{
		Tokens:     BatchToResponseNfts(tokens),
		Pagination: pageResp,
	})
}

func (h *NftHandler) buildBaseNftQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedNft{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
