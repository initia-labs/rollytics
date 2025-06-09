package tx

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

// GetEvmTxs handles GET /tx/v1/evm-txs
// @Summary Get EVM transactions
// @Description Get a list of EVM transactions with pagination
// @Tags Evm Transactions
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/tx/v1/evm-txs [get]
func (h *TxHandler) GetEvmTxs(c *fiber.Ctx) error {
	req, err := ParseEvmTxsRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseEvmTxQuery()
	query, err = req.Pagination.ApplyPagination(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var txs []dbtypes.CollectedEvmTx
	if err := query.Find(&txs).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchEvmTx)
	}

	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertEvmTx)
	}

	var nextKey uint64
	if len(txs) > 0 {
		nextKey = txs[len(txs)-1].Sequence
	}

	pageResp, err := req.Pagination.GetPageResponse(len(txs), h.buildBaseEvmTxQuery(), nextKey)
	if err != nil {
		return err
	}

	resp := EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	}

	return c.JSON(resp)
}

// GetEvmTxsByAccount handles GET /tx/v1/evm-txs/by_account/{account}
// @Summary Get EVM transactions by account
// @Description Get EVM transactions associated with a specific account
// @Tags Evm Transactions
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/tx/v1/evm-txs/by_account/{account} [get]
func (h *TxHandler) GetEvmTxsByAccount(c *fiber.Ctx) error {
	req, err := ParseEvmTxsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.Model(&dbtypes.CollectedEvmTx{}).
		Select("evm_tx.*").
		InnerJoins("evm_account_tx ON evm_tx.chain_id = evm_account_tx.chain_id AND evm_tx.hash = evm_account_tx.hash").
		Where("evm_account_tx.chain_id = ?", h.GetChainConfig().ChainId).Where("evm_account_tx.account = ?", req.Account)

	query, err = req.Pagination.ApplyPagination(query, "evm_tx.sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var txs []dbtypes.CollectedEvmTx
	if err := query.Find(&txs).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchEvmTx)
	}

	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertEvmTx)
	}

	var nextKey uint64
	if len(txs) > 0 {
		nextKey = txs[len(txs)-1].Sequence
	}

	pageResp, err := req.Pagination.GetPageResponse(len(txs), h.Model(&dbtypes.CollectedEvmAccountTx{}).
		Where("chain_id = ? AND account = ?", h.GetChainConfig().ChainId, req.Account), nextKey)
	if err != nil {
		return err
	}

	resp := EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	}

	return c.JSON(resp)
}

// GetEvmTxsByHeight handles GET /tx/v1/evm-txs/by_height/{height}
// @Summary Get EVM transactions by height
// @Description Get EVM transactions at a specific block height
// @Tags Evm Transactions
// @Accept json
// @Produce json
// @Param height path int true "Block height"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/tx/v1/evm-txs/by_height/{height} [get]
func (h *TxHandler) GetEvmTxsByHeight(c *fiber.Ctx) error {
	req, err := ParseEvmTxsByHeightRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseEvmTxQuery()
	query = query.Where("height = ?", req.Height)
	query, err = req.Pagination.ApplyPagination(query, "sequence")

	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var txs []dbtypes.CollectedEvmTx
	if err := query.Find(&txs).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txsResps, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	var nextKey uint64
	if len(txs) > 0 {
		nextKey = txs[len(txs)-1].Sequence
	}

	pageResp, err := req.Pagination.GetPageResponse(len(txs), h.Model(&dbtypes.CollectedEvmTx{}).Where("chain_id = ? AND height = ?", h.GetChainConfig().ChainId, req.Height), nextKey)
	if err != nil {
		return err
	}

	resp := EvmTxsResponse{
		Txs:        txsResps,
		Pagination: pageResp,
	}

	return c.JSON(resp)
}

// GetEvmTxsCount handles GET /tx/v1/evm-txs/count
// @Summary Get EVM transaction count
// @Description Get the total number of EVM transactions
// @Tags Evm Transactions
// @Accept json
// @Produce json
// @Success 200 {object} EvmTxCountResponse
// @Router /indexer/tx/v1/evm-txs/count [get]
func (h *TxHandler) GetEvmTxsCount(c *fiber.Ctx) error {
	var total int64
	if err := h.Model(&dbtypes.CollectedEvmTx{}).Count(&total).Error; err != nil {
		h.Logger.Error(ErrFailedToCountEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToCountEvmTx)
	}
	resp := EvmTxCountResponse{Count: uint64(total)}
	return c.JSON(resp)
}

// GetEvmTxByHash handles GET /tx/v1/evm-txs/{tx_hash}
// @Summary Get EVM transaction by hash
// @Description Get a specific EVM transaction by its hash
// @Tags Evm Transactions
// @Accept json
// @Produce json
// @Param tx_hash path string true "Transaction hash"
// @Router /indexer/tx/v1/evm-txs/{tx_hash} [get]
func (h *TxHandler) GetEvmTxByHash(c *fiber.Ctx) error {
	req, err := ParseEvmTxByHashRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseEvmTxQuery()
	query = query.Where("hash = ?", req.Hash)

	var tx dbtypes.CollectedEvmTx
	if err := query.Find(&tx).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchEvmTx)
	}

	txResp, err := ToResponseEvmTx(&tx)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertEvmTx)
	}

	resp := EvmTxResponse{
		Tx: txResp,
	}
	return c.JSON(resp)
}

func (h *TxHandler) buildBaseEvmTxQuery() *gorm.DB {
	return h.Model(&dbtypes.CollectedEvmTx{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
