package tx

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

// GetTxs handles GET /tx/v1/txs
// @Summary Get transactions
// @Description Get a list of transactions with pagination
// @Tags Transactions
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/txs [get]
func (h *TxHandler) GetTxs(c *fiber.Ctx) error {
	req, err := ParseTxsRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	txs, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedTx](req.Pagination).
		WithQuery(h.buildBaseTxQuery()).
		WithKeys("sequence").
		WithKeyExtractor(func(tx dbtypes.CollectedTx) interface{} {
			return tx.Sequence
		}).
		Execute()

	if err != nil {
		h.GetLogger().Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.GetLogger().Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	return c.JSON(TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
}

// GetTxsByAccount handles GET /tx/v1/txs/by_account/{account}
// @Summary Get transactions by account
// @Description Get transactions associated with a specific account
// @Tags Transactions
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/txs/by_account/{account} [get]
func (h *TxHandler) GetTxsByAccount(c *fiber.Ctx) error {
	req, err := ParseTxsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	chainId := h.GetChainConfig().ChainId

	query := h.GetDatabase().Model(&dbtypes.CollectedTx{}).
		Select("tx.*").
		Joins("INNER JOIN account_tx ON tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash").
		Where("account_tx.chain_id = ?", chainId).
		Where("account_tx.account = ?", req.Account)

	countQuery := h.GetDatabase().Model(&dbtypes.CollectedAccountTx{}).
		Where("chain_id = ? AND account = ?", chainId, req.Account)

	txs, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedTx](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("tx.sequence").
		WithKeyExtractor(func(tx dbtypes.CollectedTx) interface{} {
			return tx.Sequence
		}).
		Execute()

	if err != nil {
		h.GetLogger().Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.GetLogger().Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	return c.JSON(TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
}

// GetTxsByHeight handles GET /tx/v1/txs/by_height/{height}
// @Summary Get transactions by height
// @Description Get transactions at a specific block height
// @Tags Transactions
// @Accept json
// @Produce json
// @Param height path int true "Block height"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool true "Count total" default(true)
// @Param pagination.reverse query bool true "Reverse order default(true) if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/txs/by_height/{height} [get]
func (h *TxHandler) GetTxsByHeight(c *fiber.Ctx) error {
	req, err := ParseTxsRequestByHeight(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseTxQuery().
		Where("height = ?", req.Height)

	countQuery := h.GetDatabase().Model(&dbtypes.CollectedTx{}).
		Where("chain_id = ? AND height = ?", h.GetChainConfig().ChainId, req.Height)

	txs, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedTx](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("sequence").
		WithKeyExtractor(func(tx dbtypes.CollectedTx) interface{} {
			return tx.Sequence
		}).
		Execute()

	if err != nil {
		h.GetLogger().Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.GetLogger().Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	return c.JSON(TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
}

// GetTxsCount handles GET /tx/v1/txs/count
// @Summary Get transaction count
// @Description Get the total number of transactions
// @Tags Transactions
// @Accept json
// @Produce json
// @Router /indexer/tx/v1/txs/count [get]
func (h *TxHandler) GetTxsCount(c *fiber.Ctx) error {
	var total int64

	if err := h.buildBaseTxQuery().Count(&total).Error; err != nil {
		h.GetLogger().Error(ErrFailedToCountTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToCountTx)
	}

	return c.JSON(TxCountResponse{
		Count: uint64(total),
	})
}

// GetTxByHash handles GET /tx/v1/txs/{tx_hash}
// @Summary Get transaction by hash
// @Description Get a specific transaction by its hash
// @Tags Transactions
// @Accept json
// @Produce json
// @Param tx_hash path string true "Transaction hash"
// @Router /indexer/tx/v1/txs/{tx_hash} [get]
func (h *TxHandler) GetTxByHash(c *fiber.Ctx) error {
	req, err := ParseTxByHashRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var tx dbtypes.CollectedTx
	if err := h.buildBaseTxQuery().
		Where("hash = ?", req.Hash).
		First(&tx).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fiber.NewError(fiber.StatusNotFound, "Transaction not found")
		}
		h.GetLogger().Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txResp, err := ToResponseTx(&tx)
	if err != nil {
		h.GetLogger().Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	return c.JSON(TxResponse{
		Tx: txResp,
	})
}

func (h *TxHandler) buildBaseTxQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedTx{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
