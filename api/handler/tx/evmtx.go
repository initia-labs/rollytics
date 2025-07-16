package tx

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
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
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-txs [get]
func (h *TxHandler) GetEvmTxs(c *fiber.Ctx) error {
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var lastTx types.CollectedEvmTx
	if err := h.buildBaseEvmTxQuery().
		Order("sequence DESC").
		Limit(1).
		First(&lastTx).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	total := lastTx.Sequence

	var txs []types.CollectedEvmTx
	if err := h.buildBaseEvmTxQuery().
		Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToEvmTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
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
// @Param pagination.count_total query bool false "Count total, default is true" default is true
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

	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	query := h.buildBaseEvmTxQuery().Where("account_ids && ?", pq.Array(accountIds))

	if isSigner {
		query = query.Where("signer = ?", account)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmTx
	if err := query.
		Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToEvmTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
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
// @Param pagination.count_total query bool false "Count total, default is true" default is true
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

	query := h.buildBaseEvmTxQuery().Where("height = ?", height)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmTx
	if err := query.
		Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes, err := ToEvmTxsResponse(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
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
//
//nolint:dupl
func (h *TxHandler) GetEvmTxByHash(c *fiber.Ctx) error {
	hash, err := common.GetParams(c, "tx_hash")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var tx types.CollectedEvmTx
	if err := h.buildBaseEvmTxQuery().
		Where("hash = ?", hash).
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

func (h *TxHandler) buildBaseEvmTxQuery() *gorm.DB {
	return h.GetDatabase().Model(&types.CollectedEvmTx{})
}

func (h *TxHandler) NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).SendString("evm routes are not available on this chain")
}
