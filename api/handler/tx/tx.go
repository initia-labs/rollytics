package tx

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// GetTxs handles GET /tx/v1/txs
// @Summary Get transactions
// @Description Get a list of transactions with pagination
// @Tags Tx
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Param msgs query []string false "Message types to filter (comma-separated or multiple params)" collectionFormat(multi) example("cosmos.bank.v1beta1.MsgSend,initia.move.v1.MsgExecute")
// @Router /indexer/tx/v1/txs [get]
func (h *TxHandler) GetTxs(c *fiber.Ctx) error {
	req := ParseTxsRequest(c)

	var err error
	query := h.buildBaseTxQuery()
	if len(req.Msgs) > 0 {
		msgTypeIds, err := util.GetOrCreateMsgTypeIds(h.GetDatabase().DB, req.Msgs, false)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
	}
	query, err = req.Pagination.Apply(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// execute the query with pagination
	var txs []dbtypes.CollectedTx
	if err := query.Find(&txs).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "No transactions found")
		}
		h.GetLogger().Error("GetTxs", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// response
	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.GetLogger().Error("GetTxs", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	pageResp := common.GetPageResponse(req.Pagination, txs, func(tx dbtypes.CollectedTx) []any {
		return []any{tx.Sequence}
	}, func() int64 {
		var tx dbtypes.CollectedTx
		if h.buildBaseTxQuery().Select("sequence").Order("sequence DESC").First(&tx).Error != nil {
			return 0
		}
		return int64(tx.Sequence)
	})

	return c.JSON(TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
}

// GetTxsByAccount handles GET /tx/v1/txs/by_account/{account}
// @Summary Get transactions by account
// @Description Get transactions associated with a specific account
// @Tags Tx
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Param is_signer query bool false "Filter by signer accounts, default is false" default is false
// @Param msgs query []string false "Message types to filter (comma-separated or multiple params)" collectionFormat(multi) example("cosmos.bank.v1beta1.MsgSend,initia.move.v1.MsgExecute")
// @Router /indexer/tx/v1/txs/by_account/{account} [get]
func (h *TxHandler) GetTxsByAccount(c *fiber.Ctx) error {
	req, err := ParseTxsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	chainId := h.GetChainConfig().ChainId
	query := h.GetDatabase().Model(&dbtypes.CollectedTx{}).
		Select("tx.data", "account_tx.sequence as sequence").
		Joins("INNER JOIN account_tx ON tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash").
		Where("account_tx.chain_id = ?", chainId).
		Where("account_tx.account = ?", req.Account)

	totalQuery := func() int64 {
		var total int64
		h.GetDatabase().Model(&dbtypes.CollectedAccountTx{}).
			Where("chain_id = ?", chainId).
			Where("account = ?", req.Account).Count(&total)
		return total
	}

	// If the IsSigner flag is set, filter by signer accounts
	if req.IsSigner {
		query = query.Where("tx.signer = ?", req.Account)
	}
	
	// If there are message types specified, filter by them
	if len(req.Msgs) > 0 {
		msgTypeIds, err := util.GetOrCreateMsgTypeIds(h.GetDatabase().DB, req.Msgs, false)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
		totalQuery = func() int64 {
			var total int64
			h.GetDatabase().Model(&dbtypes.CollectedAccountTx{}).
				Where("chain_id = ? AND account = ?", chainId, req.Account).
				Where("EXISTS (SELECT 1 FROM tx WHERE tx.chain_id = account_tx.chain_id AND tx.hash = account_tx.hash AND msg_type_ids && ?)", pq.Array(msgTypeIds)).Count(&total)
			return total
		}
	}

	// pagination
	query, err = req.Pagination.Apply(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []dbtypes.CollectedTx
	if err := query.Find(&txs).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "No transactions found for the specified account")
		}
		h.GetLogger().Error("GetTxsByAccount", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// response
	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.GetLogger().Error("GetTxsByAccount", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	pageResp := common.GetPageResponse(req.Pagination, txs, func(tx dbtypes.CollectedTx) []any {
		return []any{tx.Sequence}
	}, totalQuery)

	return c.JSON(TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
}

// GetTxsByHeight handles GET /tx/v1/txs/by_height/{height}
// @Summary Get transactions by height
// @Description Get transactions at a specific block height
// @Tags Tx
// @Accept json
// @Produce json
// @Param height path int true "Block height"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Param msgs query []string false "Message types to filter (comma-separated or multiple params)" collectionFormat(multi) example("cosmos.bank.v1beta1.MsgSend,initia.move.v1.MsgExecute")
// @Router /indexer/tx/v1/txs/by_height/{height} [get]
func (h *TxHandler) GetTxsByHeight(c *fiber.Ctx) error {
	req, err := ParseTxsByHeightRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseTxQuery().
		Where("height = ?", req.Height)

	totalQuery := func() int64 {
		var block dbtypes.CollectedBlock
		if h.GetDatabase().Model(&dbtypes.CollectedBlock{}).
			Where("chain_id = ? AND height = ?", h.GetChainConfig().ChainId, req.Height).First(&block).Error != nil {
			return 0
		}
		return int64(block.TxCount)
	}

	if len(req.Msgs) > 0 {
		msgTypeIds, err := util.GetOrCreateMsgTypeIds(h.GetDatabase().DB, req.Msgs, false)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
		totalQuery = func() int64 {
			var count int64
			if query.Count(&count).Error != nil {
				return 0
			}
			return count
		}
	}

	query, err = req.Pagination.Apply(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []dbtypes.CollectedTx
	if err := query.Find(&txs).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "No transactions found for the specified height")
		}
		h.GetLogger().Error("GetTxsByHeight", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsResp, err := BatchToResponseTxs(txs)
	if err != nil {
		h.GetLogger().Error("GetTxsByHeight", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	pageResp := common.GetPageResponse(req.Pagination, txs, func(tx dbtypes.CollectedTx) []any {
		return []any{tx.Sequence}
	}, totalQuery)

	return c.JSON(TxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
}

// GetTxByHash handles GET /tx/v1/txs/{tx_hash}
// @Summary Get transaction by hash
// @Description Get a specific transaction by its hash
// @Tags Tx
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Transaction not found")
		}
		h.GetLogger().Error("GetTxByHash", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txResp, err := ToResponseTx(&tx)
	if err != nil {
		h.GetLogger().Error("GetTxByHash", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(TxResponse{
		Tx: txResp,
	})
}

func (h *TxHandler) buildBaseTxQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedTx{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
