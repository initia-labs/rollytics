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

// GetEvmTxs handles GET /tx/v1/evm-txs
// @Summary Get EVM transactions
// @Description Get a list of EVM transactions with pagination
// @Tags EVM Tx
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-txs [get]
func (h *TxHandler) GetEvmTxs(c *fiber.Ctx) error {
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	// Use optimized COUNT - no filters
	query := tx.Model(&types.CollectedEvmTx{})
	var strategy types.CollectedEvmTx
	hasFilters := false // no filters in basic GetEvmTxs
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmTx
	findQuery := pagination.ApplyToEvmTx(tx.Model(&types.CollectedEvmTx{}))
	if err := findQuery.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToEvmTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(txs) > 0 {
		lastRecord = txs[len(txs)-1]
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
	})
}

// GetEvmTxsByAccount handles GET /tx/v1/evm-txs/by_account/{account}
// @Summary Get EVM transactions by account
// @Description Get EVM transactions associated with a specific account
// @Tags EVM Tx
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Param is_signer query bool false "Filter by signer accounts, default is false" default is false
// @Router /indexer/tx/v1/evm-txs/by_account/{account} [get]
func (h *TxHandler) GetEvmTxsByAccount(c *fiber.Ctx) error {
	account, err := common.GetAccountParam(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
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
		return c.JSON(EvmTxsResponse{
			Txs:        []types.EvmTx{},
			Pagination: pagination.ToResponse(0),
		})
	}
	query := tx.Model(&types.CollectedEvmTx{}).Where("account_ids && ?", pq.Array(accountIds))

	if isSigner {
		query = query.Where("signer_id = ?", accountIds[0])
	}

	// Use optimized COUNT - always has filters (account_ids + optional signer)
	var strategy types.CollectedEvmTx
	hasFilters := true // always has account_ids filter
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmTx
	finalQuery := pagination.ApplyToEvmTxWithFilter(query)
	if err := finalQuery.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToEvmTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(txs) > 0 {
		lastRecord = txs[len(txs)-1]
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
	})
}

// GetEvmTxsByHeight handles GET /tx/v1/evm-txs/by_height/{height}
// @Summary Get EVM transactions by height
// @Description Get EVM transactions at a specific block height
// @Tags EVM Tx
// @Accept json
// @Produce json
// @Param height path int true "Block height"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-txs/by_height/{height} [get]
func (h *TxHandler) GetEvmTxsByHeight(c *fiber.Ctx) error {
	height, err := common.GetHeightParam(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	query := tx.Model(&types.CollectedEvmTx{}).Where("height = ?", height)

	// Use optimized COUNT - always has filters (height)
	var strategy types.CollectedEvmTx
	hasFilters := true // always has height filter
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmTx
	finalQuery := pagination.ApplyToEvmTxWithFilter(query)
	if err := finalQuery.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToEvmTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var lastRecord any
	if len(txs) > 0 {
		lastRecord = txs[len(txs)-1]
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
	})
}

// GetEvmTxByHash handles GET /tx/v1/evm-txs/{tx_hash}
// @Summary Get EVM transaction by hash
// @Description Get a specific EVM transaction by its hash
// @Tags EVM Tx
// @Accept json
// @Produce json
// @Param tx_hash path string true "Transaction hash"
// @Router /indexer/tx/v1/evm-txs/{tx_hash} [get]
func (h *TxHandler) GetEvmTxByHash(c *fiber.Ctx) error {
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

	var tx types.CollectedEvmTx
	if err := dbTx.Model(&types.CollectedEvmTx{}).
		Where("hash = ?", hashBytes).
		First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "tx not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txRes, err := ToEvmTxResponse(tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(EvmTxResponse{
		Tx: txRes,
	})
}

func (h *TxHandler) NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).SendString("evm routes are not available on this chain")
}
