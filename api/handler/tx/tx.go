package tx

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
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
	msgs := common.GetMsgsQuery(c)
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseTxQuery()

	var total int64
	if len(msgs) > 0 {
		msgTypeIds, err := h.GetMsgTypeIds(msgs)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
		if err := query.Count(&total).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	} else {
		var lastTx types.CollectedTx
		if err := h.buildBaseTxQuery().
			Order("sequence DESC").
			Limit(1).
			First(&lastTx).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		total = lastTx.Sequence
	}

	var txs []types.CollectedTx
	if err := query.
		Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(TxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
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
	account, err := common.GetAccountParam(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	msgs := common.GetMsgsQuery(c)
	isSigner := c.Query("is_signer", "false") == "true"
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	query := h.buildBaseTxQuery().Where("account_ids && ?", pq.Array(accountIds))

	if len(msgs) > 0 {
		msgTypeIds, err := h.GetMsgTypeIds(msgs)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
	}

	if isSigner {
		query = query.Where("signer = ?", account)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedTx
	if err := query.
		Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(TxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
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
	height, err := common.GetHeightParam(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	msgs := common.GetMsgsQuery(c)
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseTxQuery().Where("height = ?", height)

	if len(msgs) > 0 {
		msgTypeIds, err := h.GetMsgTypeIds(msgs)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedTx
	if err := query.
		Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(TxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
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
//
//nolint:dupl
func (h *TxHandler) GetTxByHash(c *fiber.Ctx) error {
	hash, err := common.GetParams(c, "tx_hash")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var tx types.CollectedTx
	if err := h.buildBaseTxQuery().
		Where("hash = ?", hash).
		First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "tx not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txRes, err := ToTxResponse(&tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(TxResponse{
		Tx: txRes,
	})
}

func (h *TxHandler) buildBaseTxQuery() *gorm.DB {
	return h.GetDatabase().Model(&types.CollectedTx{})
}
