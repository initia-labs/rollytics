package nft

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/api/handler/tx"
	"github.com/initia-labs/rollytics/types"
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
	req, err := ParseNftTxsRequest(h.GetChainConfig(), c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	chainConfig := h.GetChainConfig()
	chainId := chainConfig.ChainId

	var query *gorm.DB
	var totalQuery func() int64

	if chainConfig.VmType == types.MoveVM {
		// Move VM: get NFT address and use account transactions
		var nft types.CollectedNft
		if err := h.GetDatabase().Model(&types.CollectedNft{}).
			Where("chain_id = ? AND collection_addr = ? AND token_id = ?", chainId, req.CollectionAddr, req.TokenId).
			First(&nft).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiber.NewError(fiber.StatusNotFound, "NFT not found")
			}
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		query = h.GetDatabase().Model(&types.CollectedTx{}).
			Select("tx.data", "account_tx.sequence as sequence").
			Joins("INNER JOIN account_tx ON tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash").
			Where("account_tx.chain_id = ?", chainId).
			Where("account_tx.account = ?", nft.Addr)

		totalQuery = func() int64 {
			var total int64
			if h.GetDatabase().Model(&types.CollectedAccountTx{}).
				Where("chain_id = ? AND account = ?", chainId, nft.Addr).Count(&total).Error != nil {
				return 0
			}
			return total
		}
	} else {
		// EVM/WASM: use nft_tx table
		query = h.GetDatabase().Model(&types.CollectedTx{}).
			Select("tx.data", "tx.sequence as sequence").
			Joins("INNER JOIN nft_tx ON tx.chain_id = nft_tx.chain_id AND tx.hash = nft_tx.hash").
			Where("nft_tx.chain_id = ?", chainId).
			Where("nft_tx.collection_addr = ?", req.CollectionAddr).
			Where("nft_tx.token_id = ?", req.TokenId)

		totalQuery = func() int64 {
			var total int64
			if h.GetDatabase().Model(&types.CollectedNftTx{}).
				Where("chain_id = ? AND collection_addr = ? AND token_id = ?", chainId, req.CollectionAddr, req.TokenId).Count(&total).Error != nil {
				return 0
			}
			return total
		}
	}

	// pagination
	query, err = req.Pagination.Apply(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var nftTxs []types.CollectedTx
	if err := query.Find(&nftTxs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// response
	pageResp := common.GetPageResponse(req.Pagination, nftTxs, func(tx types.CollectedTx) []any {
		return []any{tx.Sequence}
	}, totalQuery)

	txsResp, err := tx.BatchToResponseTxs(nftTxs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(tx.TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
}
