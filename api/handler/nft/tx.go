package nft

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/api/handler/tx"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
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

	chainId := h.Config.GetChainConfig().ChainId

	var query *gorm.DB
	var totalQuery *gorm.DB
	if h.Config.GetChainConfig().VmType == types.MoveVM {
		// get move nft address
		// no nft_tx table in move vm
		// so we need to get the nft address from collected_nft table
		nftQuery := h.Model(&types.CollectedNft{}).Where("chain_id = ? AND collection_addr = ? AND token_id = ?",
			chainId, req.CollectionAddr, req.TokenId)

		var nft types.CollectedNft
		if err := nftQuery.First(&nft).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fiber.NewError(fiber.StatusNotFound, ErrFailedToFetchNft)
			}
			return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNft)
		}

		query = h.Model(&types.CollectedTx{}).Select("tx.*").
			Joins("INNER JOIN account_tx ON tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash").
			Where("account_tx.chain_id = ?", chainId).
			Where("account_tx.account = ?", nft.Addr)

		totalQuery = h.Model(&types.CollectedAccountTx{}).
			Where("chain_id = ?", chainId).
			Where("account = ?", nft.Addr)

	} else {
		query = h.Model(&types.CollectedTx{}).Select("tx.*").
			Joins("INNER JOIN nft_tx ON tx.chain_id = nft_tx.chain_id AND tx.hash = nft_tx.hash").
			Where("nft_tx.chain_id = ?", chainId).
			Where("nft_tx.collection_addr = ?", req.CollectionAddr).
			Where("nft_tx.token_id = ?", req.TokenId)

		totalQuery = h.Model(&types.CollectedNftTx{}).
			Where("chain_id = ?", chainId).
			Where("collection_addr = ?", req.CollectionAddr).
			Where("token_id = ?", req.TokenId)
	}

	query, err = req.Pagination.ApplyPagination(query, "tx.sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}
	var nftTxs []types.CollectedTx
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

	pageResp, err := req.Pagination.GetPageResponse(len(nftTxs), totalQuery, nextKey)
	if err != nil {
		return err
	}
	resp := tx.TxsResponse{
		Txs:        txResps,
		Pagination: pageResp,
	}
	return c.JSON(resp)
}
