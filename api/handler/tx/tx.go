package tx

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

type TxHandler struct {
	*common.Handler
}

// GetTxs handles GET /tx/v1/
// @Summary Get transactions
// @Description Get a list of transactions with pagination
// @Tags Transactions
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/tx/v1/txs [get]
func (h *TxHandler) GetTxs(c *fiber.Ctx) error {
	req, err := ParseTxsRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseTxQuery()
	query, err = req.Pagination.ApplyPagination(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var txs []dbtypes.CollectedTx
	if err := query.Find(&txs).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	var nextKey uint64
	if len(txs) > 0 {
		nextKey = txs[len(txs)-1].Sequence
	}

	pageResp, err := req.Pagination.GetPageResponse(len(txs), h.buildBaseTxQuery(), nextKey)
	if err != nil {
		return err
	}
	resp := TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	}

	return c.JSON(resp)
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
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/tx/v1/txs/by_account/{account} [get]
func (h *TxHandler) GetTxsByAccount(c *fiber.Ctx) error {
	req, err := ParseTxsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var (
		query    *gorm.DB
		accounts = []string{req.Account}
		chainId  = h.GetChainConfig().ChainId
	)

	// in move vm, we also need to query the FA store addresses
	if h.GetChainConfig().VmType == dbtypes.MoveVM {
		var storeAddrs []string
		if err := h.Model(&dbtypes.CollectedFAStore{}).
			Where("chain_id = ? AND owner = ?", chainId, req.Account).
			Pluck("store_addr", &storeAddrs).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
		}
		accounts = append(accounts, storeAddrs...)
	}
	query = h.Model(&dbtypes.CollectedTx{}).Select("tx.*").
		InnerJoins("account_tx ON tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash").
		Where("account_tx.chain_id = ?", chainId).
		Where("account_tx.account IN ?", accounts)
	query, err = req.Pagination.ApplyPagination(query, "tx.sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var txs []dbtypes.CollectedTx
	if err := query.Find(&txs).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	var nextKey uint64
	if len(txs) > 0 {
		nextKey = txs[len(txs)-1].Sequence
	}

	pageResp, err := req.Pagination.GetPageResponse(len(txs), h.Model(&dbtypes.CollectedAccountTx{}).Where("chain_id = ? AND account = ?", chainId, req.Account), nextKey)
	if err != nil {
		return err
	}
	resp := TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	}

	return c.JSON(resp)
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
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/tx/v1/txs/by_height/{height} [get]
func (h *TxHandler) GetTxsByHeight(c *fiber.Ctx) error {
	req, err := ParseTxsRequestByHeight(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseTxQuery()
	query = query.Where("height = ?", req.Height)
	query, err = req.Pagination.ApplyPagination(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var txs []dbtypes.CollectedTx
	if err := query.Find(&txs).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txsResps, err := BatchToResponseTxs(txs)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	var nextKey uint64
	if len(txs) > 0 {
		nextKey = txs[len(txs)-1].Sequence
	}

	pageResp, err := req.Pagination.GetPageResponse(len(txs), h.Model(&dbtypes.CollectedTx{}).Where("chain_id = ? AND height = ?", h.GetChainConfig().ChainId, req.Height), nextKey)
	if err != nil {
		return err
	}

	resp := TxsResponse{
		Txs:        txsResps,
		Pagination: pageResp,
	}

	return c.JSON(resp)
}

// GetTxsCount handles GET /tx/v1/txs/count
// @Summary Get transaction count
// @Description Get the total number of transactions
// @Tags Transactions
// @Accept json
// @Produce json
// @Router /indexer/tx/v1/txs/count [get]
func (h *TxHandler) GetTxsCount(c *fiber.Ctx) error {
	var lastTx dbtypes.CollectedTx
	if err := h.Model(&dbtypes.CollectedTx{}).Order("sequence DESC").First(&lastTx).Error; err != nil {
		h.Logger.Error(ErrFailedToCountTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToCountTx)
	}
	resp := TxCountResponse{Count: uint64(lastTx.Sequence)}
	return c.JSON(resp)
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

	query := h.buildBaseTxQuery()
	query = query.Where("hash = ?", req.Hash)

	var tx dbtypes.CollectedTx
	if err := query.Find(&tx).Error; err != nil {
		h.Logger.Error(ErrFailedToFetchTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchTx)
	}

	txResp, err := ToResponseTx(&tx)
	if err != nil {
		h.Logger.Error(ErrFailedToConvertTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertTx)
	}

	resp := TxResponse{
		Tx: txResp,
	}
	return c.JSON(resp)
}

func (h *TxHandler) buildBaseTxQuery() *gorm.DB {
	return h.Model(&dbtypes.CollectedTx{}).Where("chain_id = ?", h.Config.GetChainConfig().ChainId)
}
