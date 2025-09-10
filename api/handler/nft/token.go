package nft

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

// normalizeOrderBy validates and returns a normalized (lowercased, trimmed) order_by.
func normalizeOrderBy(orderBy string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(orderBy))
	// default to token_id
	if v == "" {
		return "token_id", nil
	}
	switch v {
	case "token_id", "height":
		return v, nil
	default:
		return "", fmt.Errorf("invalid order_by value '%s', must be one of: token_id, height", orderBy)
	}
}

// getTokensWithFilters is a shared function that handles the common logic for fetching NFTs
// with various filters and pagination
func (h *NftHandler) getTokensWithFilters(
	tx *gorm.DB,
	baseQuery *gorm.DB,
	pagination *common.Pagination,
	orderBy string,
) (*NftsResponse, error) {
	// Use optimized COUNT
	var strategy types.CollectedNft
	hasFilters := true // always has base filters
	total, err := common.GetOptimizedCount(baseQuery, strategy, hasFilters)
	if err != nil {
		return nil, err
	}

	var nfts []types.CollectedNft
	if err := pagination.ApplyToNft(baseQuery, orderBy).Find(&nfts).Error; err != nil {
		return nil, err
	}

	ownerAccounts, err := h.getNftOwnerIdMap(tx, nfts)
	if err != nil {
		return nil, err
	}

	nftsRes, err := ToNftsResponse(h.GetDatabase(), nfts, ownerAccounts)
	if err != nil {
		return nil, err
	}

	var lastRecord any
	if len(nfts) > 0 {
		lastRecord = nfts[len(nfts)-1]
	}

	return &NftsResponse{
		Tokens:     nftsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
	}, nil
}

// GetTokensByAccount handles GET /nft/v1/tokens/by_account/{account}
// @Summary Get NFT tokens by account
// @Description Get NFT tokens owned by a specific account
// @Tags NFT
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param collection_addr query string false "Collection address to filter by (optional)"
// @Param token_id query string false "Token ID to filter by (optional)"
// @Param order_by query string false "Order by field (token_id, height)"
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

	orderBy, err := normalizeOrderBy(c.Query("order_by"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	// Get account ID from account_dict
	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if len(accountIds) == 0 {
		return c.JSON(NftsResponse{
			Tokens:     []Nft{},
			Pagination: pagination.ToResponse(0),
		})
	}

	query := tx.Model(&types.CollectedNft{}).Where("owner_id = ?", accountIds[0])

	if collectionAddr != nil {
		query = query.Where("collection_addr = ?", collectionAddr)
	}
	if tokenId != "" {
		query = query.Where("token_id = ?", tokenId)
	}

	response, err := h.getTokensWithFilters(tx, query, pagination, orderBy)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(response)
}

// GetTokensByCollectionAddr handles GET /nft/v1/tokens/by_collection/{collection_addr}
// @Summary Get NFT tokens by collection
// @Description Get NFT tokens from a specific collection
// @Tags NFT
// @Accept json
// @Produce json
// @Param collection_addr path string true "Collection address"
// @Param token_id query string false "Token ID to filter by (optional)"
// @Param order_by query string false "Order by field (token_id, height)"
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

	orderBy, err := normalizeOrderBy(c.Query("order_by"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	query := tx.Model(&types.CollectedNft{}).Where("collection_addr = ?", collectionAddr)

	if tokenId != "" {
		query = query.Where("token_id = ?", tokenId)
	}

	response, err := h.getTokensWithFilters(tx, query, pagination, orderBy)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(response)
}
