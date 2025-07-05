package nft

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/api/handler/tx"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
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
		Where("chain_id = ? AND collection_addr = ? AND token_id = ?", h.GetChainId(), collectionAddr, tokenId).
		First(&nft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(tx.TxsResponse{
				Txs:        []types.Tx{},
				Pagination: pagination.ToResponse(0),
			})
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	query := h.GetDatabase().
		Model(&types.CollectedTx{}).
		Where("tx.chain_id = ?", h.GetChainId()).
		Order(pagination.OrderBy("sequence"))

	if h.GetVmType() == types.MoveVM {
		accAddr, err := util.AccAddressFromString(nft.Addr)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		query = query.
			Joins("INNER JOIN account_tx ON tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash").
			Where("account_tx.account = ?", accAddr.String())
	} else {
		query = query.
			Joins("INNER JOIN nft_tx ON tx.chain_id = nft_tx.chain_id AND tx.hash = nft_tx.hash").
			Where("nft_tx.collection_addr = ? AND nft_tx.token_id = ?", collectionAddr, tokenId)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
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
		Pagination: pagination.ToResponse(total),
	})
}
