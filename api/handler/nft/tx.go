package nft

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/api/handler/tx"
	dbtypes "github.com/initia-labs/rollytics/types"
)

// nft txs
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
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/nft/v1/txs/{collection_addr}/{token_id} [get]
func (h *NftHandler) GetNftTxs(c *fiber.Ctx) error {
	req, err := ParseNftTxsRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	query := h.Model(&dbtypes.CollectedTx{}).Select("tx.*").
		InnerJoins("nft_tx ON tx.chain_id = nft_tx.chain_id AND tx.hash = nft_tx.hash").
		Where("nft_tx.chain_id = ?", h.Config.GetChainConfig().ChainId).
		Where("nft_tx.collection_addr = ?", req.CollectionAddr).
		Where("nft_tx.token_id = ?", req.TokenId)
	query, err = req.Pagination.ApplyPagination(query, "tx.sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}
	var nftTxs []dbtypes.CollectedTx
	if err := query.Find(&nftTxs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNftTxs)
	}
	txResps, err := tx.BatchToResponseTxs(nftTxs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, tx.ErrFailedToConvertTx)
	}
	var nextKey []byte
	if len(nftTxs) > 0 {
		nextKey = common.GetNextKey(nftTxs[len(nftTxs)-1].Height, nftTxs[len(nftTxs)-1].Hash)
	}
	pageResp, err := req.Pagination.GetPageResponse(len(nftTxs), h.Model(&dbtypes.CollectedNftTx{}).Where("collection_addr = ? AND token_id = ?", req.CollectionAddr, req.TokenId), nextKey)
	if err != nil {
		return err
	}
	resp := tx.TxsResponse{
		Txs:        txResps,
		Pagination: pageResp,
	}
	return c.JSON(resp)
}
