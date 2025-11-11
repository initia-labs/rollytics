package nft

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/tx"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

// GetNftTxs handles GET "/txs/:collection_addr/:token_id"
// @Summary Get NFT transactions by collection and token ID
// @Description Get NFT transactions for a specific collection and token ID
// @Tags NFT
// @Accept json
// @Produce json
// @Param collection_addr path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/nft/v1/txs/{collection_addr}/{token_id} [get]
func (h *NftHandler) GetNftTxs(c *fiber.Ctx) error {
	collectionAddr, err := common.GetCollectionAddrParam(c, h.GetChainConfig())
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	tokenId, err := common.GetParams(c, "token_id")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var nft types.CollectedNft
	if err := h.GetDatabase().
		Where("collection_addr = ? AND token_id = ?", collectionAddr, tokenId).
		First(&nft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(tx.TxsResponse{
				Txs:        []types.Tx{},
				Pagination: pagination.ToResponse(0, false),
			})
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	query := h.GetDatabase().Model(&types.CollectedTx{}).Order(pagination.OrderBy("sequence"))

	switch h.GetVmType() {
	case types.MoveVM:
		accAddr := sdk.AccAddress(nft.Addr)
		accountIds, err := h.GetAccountIds([]string{accAddr.String()})
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if len(accountIds) == 0 {
			return c.JSON(tx.TxsResponse{
				Txs:        []types.Tx{},
				Pagination: pagination.ToResponse(0, false),
			})
		}

		sequenceSubQuery := h.GetDatabase().
			Model(&types.CollectedTxAccount{}).
			Select("sequence").
			Where("account_id = ?", accountIds[0])
		query = query.Where("sequence IN (?)", sequenceSubQuery)

	case types.WasmVM, types.EVM:
		nftKey := util.NftKey{
			CollectionAddr: util.BytesToHexWithPrefixIfPresent(nft.CollectionAddr),
			TokenId:        nft.TokenId,
		}
		nftIds, err := h.GetNftIds([]util.NftKey{nftKey})
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if len(nftIds) == 0 {
			return c.JSON(tx.TxsResponse{
				Txs:        []types.Tx{},
				Pagination: pagination.ToResponse(0, false),
			})
		}

		sequenceSubQuery := h.GetDatabase().
			Model(&types.CollectedTxNft{}).
			Select("sequence").
			Where("nft_id IN ?", nftIds)
		query = query.Where("sequence IN (?)", sequenceSubQuery)
	}

	// Use optimized COUNT - always has filters (account_ids or nft_ids)
	var strategy types.CollectedTx
	hasFilters := true // always has account_ids or nft_ids filter
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters, pagination.CountTotal)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedTx
	if err := query.
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := tx.ToTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(tx.TxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total, len(txs) == pagination.Limit),
	})
}
