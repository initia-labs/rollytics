package nft

import (
    "github.com/gofiber/fiber/v2"
    "github.com/initia-labs/rollytics/api/handler/common"
    "github.com/initia-labs/rollytics/api/handler/tx"
    "github.com/initia-labs/rollytics/types"
    "gorm.io/gorm"
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
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Router /indexer/nft/v1/txs/{collection_addr}/{token_id} [get]
func (h *NftHandler) GetNftTxs(c *fiber.Ctx) error {
    req, err := ParseNftTxsRequest(c)
    if err != nil {
        return fiber.NewError(fiber.StatusBadRequest, err.Error())
    }

    chainConfig := h.GetChainConfig()
    chainId := chainConfig.ChainId

    var query *gorm.DB
    var countQuery *gorm.DB

    if chainConfig.VmType == types.MoveVM {
        // Move VM: get NFT address and use account transactions
        var nft types.CollectedNft
        if err := h.GetDatabase().Model(&types.CollectedNft{}).
            Where("chain_id = ? AND collection_addr = ? AND token_id = ?", chainId, req.CollectionAddr, req.TokenId).
            First(&nft).Error; err != nil {
            if err == gorm.ErrRecordNotFound {
                return fiber.NewError(fiber.StatusNotFound, ErrFailedToFetchNft)
            }
            return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNft)
        }

        query = h.GetDatabase().Model(&types.CollectedTx{}).
            Select("tx.*").
            Joins("INNER JOIN account_tx ON tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash").
            Where("account_tx.chain_id = ?", chainId).
            Where("account_tx.account = ?", nft.Addr)

        countQuery = h.GetDatabase().Model(&types.CollectedAccountTx{}).
            Where("chain_id = ? AND account = ?", chainId, nft.Addr)
    } else {
        // EVM/WASM: use nft_tx table
        query = h.GetDatabase().Model(&types.CollectedTx{}).
            Select("tx.*").
            Joins("INNER JOIN nft_tx ON tx.chain_id = nft_tx.chain_id AND tx.hash = nft_tx.hash").
            Where("nft_tx.chain_id = ?", chainId).
            Where("nft_tx.collection_addr = ?", req.CollectionAddr).
            Where("nft_tx.token_id = ?", req.TokenId)

        countQuery = h.GetDatabase().Model(&types.CollectedNftTx{}).
            Where("chain_id = ? AND collection_addr = ? AND token_id = ?", chainId, req.CollectionAddr, req.TokenId)
    }

    nftTxs, pageResp, err := common.NewPaginationBuilder[types.CollectedTx](req.Pagination).
        WithQuery(query).
        WithCountQuery(countQuery).
        WithKeys("tx.sequence").
        WithKeyExtractor(func(tx types.CollectedTx) interface{} {
            return tx.Sequence
        }).
        Execute()

    if err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchNftTxs)
    }

    txsResp, err := tx.BatchToResponseTxs(nftTxs)
    if err != nil {
        return fiber.NewError(fiber.StatusInternalServerError, tx.ErrFailedToConvertTx)
    }

    return c.JSON(tx.TxsResponse{
        Txs:        txsResp,
        Pagination: pageResp,
    })
}