package tx

import (
	"database/sql"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
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
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Param msgs query []string false "Message types to filter (comma-separated or multiple params)" collectionFormat(multi) example("cosmos.bank.v1beta1.MsgSend,initia.move.v1.MsgExecute")
// @Router /indexer/tx/v1/txs [get]
func (h *TxHandler) GetTxs(c *fiber.Ctx) error {
	msgs := common.GetMsgsQuery(c)
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	query := tx.Model(&types.CollectedTx{})

	var total int64
	hasFilters := len(msgs) > 0

	if hasFilters {
		msgTypeIds, err := h.GetMsgTypeIds(msgs)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
	}

	// Use optimized COUNT with CollectedTx strategy
	var strategy types.CollectedTx
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedTx
	var finalQuery *gorm.DB
	if len(msgs) > 0 {
		// With filters
		finalQuery = pagination.ApplyToTxWithFilter(query)
	} else {
		// Without filters
		finalQuery = pagination.ApplyToTx(query)
	}

	if err := finalQuery.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(txs) > 0 {
		lastRecord = txs[len(txs)-1]
	}

	return c.JSON(TxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
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

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if len(accountIds) == 0 {
		return c.JSON(TxsResponse{
			Txs:        []types.Tx{},
			Pagination: pagination.ToResponse(0),
		})
	}

	// Check if we can use edges for this query
	useEdges, err := common.IsEdgeBackfillReady(tx, types.SeqInfoTxEdgeBackfill)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var query *gorm.DB
	if useEdges {
		sequenceSubQuery := tx.
			Model(&types.CollectedTxAccount{}).
			Select("sequence").
			Where("account_id = ?", accountIds[0])

		if isSigner {
			sequenceSubQuery = sequenceSubQuery.Where("signer")
		}

		query = tx.Model(&types.CollectedTx{}).
			Where("sequence IN (?)", sequenceSubQuery)
	} else {
		query = tx.Model(&types.CollectedTx{}).
			Where("account_ids && ?", pq.Array(accountIds))

		if isSigner {
			query = query.Where("signer_id = ?", accountIds[0])
		}
	}

	if len(msgs) > 0 {
		msgTypeIds, err := h.GetMsgTypeIds(msgs)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
	}

	// Use optimized COUNT - always has filters (account_ids)
	var strategy types.CollectedTx
	hasFilters := true // always has account_ids filter
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedTx
	finalQuery := pagination.ApplyToTxWithFilter(query)
	if err := finalQuery.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(txs) > 0 {
		lastRecord = txs[len(txs)-1]
	}

	return c.JSON(TxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
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

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	query := tx.Model(&types.CollectedTx{}).Where("height = ?", height)

	hasFilters := true // always has height filter
	if len(msgs) > 0 {
		msgTypeIds, err := h.GetMsgTypeIds(msgs)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		query = query.Where("msg_type_ids && ?", pq.Array(msgTypeIds))
	}

	// Use optimized COUNT - always has filters (height + optional msgs)
	var strategy types.CollectedTx
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedTx
	finalQuery := pagination.ApplyToTxWithFilter(query)
	if err := finalQuery.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(txs) > 0 {
		lastRecord = txs[len(txs)-1]
	}

	return c.JSON(TxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
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
	hash, err := common.GetParams(c, "tx_hash")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	hashBytes, err := util.HexToBytes(hash)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid hash format")
	}

	// Use read-only transaction for better performance
	dbTx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer dbTx.Rollback()

	var tx types.CollectedTx
	if err := dbTx.Model(&types.CollectedTx{}).
		Where("hash = ?", hashBytes).
		First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, types.NewNotFoundError("transaction").Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, types.NewDatabaseError("get transaction", err).Error())
	}

	txRes, err := ToTxResponse(tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(TxResponse{
		Tx: txRes,
	})
}
