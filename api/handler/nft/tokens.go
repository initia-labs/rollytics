package nft

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"

	dbtypes "github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)


// tokens
// GetTokensByAccount handles GET /nft/v1/tokens/by_account/{account}
// @Summary Get NFT tokens by account
// @Description Get NFT tokens owned by a specific account
// @Tags NFT
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_account/{account} [get]
func (h *NftHandler) GetTokensByAccount(c *fiber.Ctx) error {
	req, err := ParseTokensByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseNftQuery().Where("owner = ?", req.Account)
	query, err = req.Pagination.ApplyPagination(query, "collection_addr", "token_id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var tokens []dbtypes.CollectedNft
	if err := query.Where("owner = ?", req.Account).Find(&tokens).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNft)
	}

	var nextKey []byte
	if len(tokens) > 0 {
		token := tokens[len(tokens)-1]
		nextKey = common.GetNextKey(token.CollectionAddr, token.TokenId)
	}

	pageResp, err := req.Pagination.GetPageResponse(len(tokens), h.buildBaseNftQuery().Where("owner = ?", req.Account), nextKey)
	if err != nil {
		return err
	}

	resp := NftsResponse{
		Tokens:     BatchToResponseNfts(tokens),
		Pagination: pageResp,
	}

	return c.JSON(resp)
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
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Success 200 {object} NftsResponse
// @Router /indexer/nft/v1/tokens/by_collection/{collection_addr} [get]
func (h *NftHandler) GetTokensByCollection(c *fiber.Ctx) error {
	req, err := ParseTokensByCollectionRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseNftQuery().Where("collection_addr = ?", req.CollectionAddr)
	query, err = req.Pagination.ApplyPagination(query, "token_id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}
	var tokens []dbtypes.CollectedNft
	if err := query.Find(&tokens).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNft)
	}

	var nextKey string
	if len(tokens) > 0 {
		nextKey = tokens[len(tokens)-1].TokenId
	}

	pageResp, err := req.Pagination.GetPageResponse(len(tokens), h.buildBaseNftQuery().Where("collection_addr = ?", req.CollectionAddr), nextKey)
	if err != nil {
		return err
	}

	resp := NftsResponse{
		Tokens:     BatchToResponseNfts(tokens),
		Pagination: pageResp,
	}
	return c.JSON(resp)
}

func (h *NftHandler) buildBaseNftQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedNft{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
