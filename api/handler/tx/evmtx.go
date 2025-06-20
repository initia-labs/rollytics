package tx

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
)

// GetEvmTxs handles GET /tx/v1/evm-txs
// @Summary Get EVM transactions
// @Description Get a list of EVM transactions with pagination
// @Tags Evm Transactions
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-txs [get]
func (h *TxHandler) GetEvmTxs(c *fiber.Ctx) error {
	req, err := ParseEvmTxsRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	txs, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedEvmTx](req.Pagination).
		WithQuery(h.buildBaseEvmTxQuery()).
		WithKeys("sequence").
		WithKeyExtractor(func(lastTx dbtypes.CollectedEvmTx) []any {
			return []any{lastTx.Sequence}
		}).
		Execute()

	if err != nil {
		h.GetLogger().Error(ErrFailedToFetchEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchEvmTx)
	}

	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		h.GetLogger().Error(ErrFailedToConvertEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertEvmTx)
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
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
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-txs/by_account/{account} [get]
func (h *TxHandler) GetEvmTxsByAccount(c *fiber.Ctx) error {
	req, err := ParseEvmTxsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.GetDatabase().Model(&dbtypes.CollectedEvmTx{}).
		Select("evm_tx.*").
		Joins("INNER JOIN evm_account_tx ON evm_tx.chain_id = evm_account_tx.chain_id AND evm_tx.hash = evm_account_tx.hash").
		Where("evm_account_tx.chain_id = ?", h.GetChainConfig().ChainId).
		Where("evm_account_tx.account = ?", req.Account)

	countQuery := h.GetDatabase().Model(&dbtypes.CollectedEvmAccountTx{}).
		Where("chain_id = ? AND account = ?", h.GetChainConfig().ChainId, req.Account)

	txs, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedEvmTx](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("evm_tx.sequence").
		WithKeyExtractor(func(tx dbtypes.CollectedEvmTx) []any {
			return []any{tx.Sequence}
		}).
		Execute()

	if err != nil {
		h.GetLogger().Error(ErrFailedToFetchEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchEvmTx)
	}

	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		h.GetLogger().Error(ErrFailedToConvertEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertEvmTx)
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
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
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-txs/by_height/{height} [get]
func (h *TxHandler) GetEvmTxsByHeight(c *fiber.Ctx) error {
	req, err := ParseEvmTxsByHeightRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseEvmTxQuery().
		Where("height = ?", req.Height)

	countQuery := h.GetDatabase().Model(&dbtypes.CollectedEvmTx{}).
		Where("chain_id = ? AND height = ?", h.GetChainConfig().ChainId, req.Height)

	txs, pageResp, err := common.NewPaginationBuilder[dbtypes.CollectedEvmTx](req.Pagination).
		WithQuery(query).
		WithCountQuery(countQuery).
		WithKeys("sequence").
		WithKeyExtractor(func(tx dbtypes.CollectedEvmTx) []any {
			return []any{tx.Sequence}
		}).
		Execute()

	if err != nil {
		h.GetLogger().Error(ErrFailedToFetchEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchEvmTx)
	}

	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		h.GetLogger().Error(ErrFailedToConvertEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertEvmTx)
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
	})
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
	var (
		logger = h.GetLogger()
	)
	req, err := ParseEvmTxByHashRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseEvmTxQuery()
	query = query.Where("hash = ?", req.Hash)

	var tx dbtypes.CollectedEvmTx
	if err := query.Find(&tx).Error; err != nil {
		logger.Error(ErrFailedToFetchEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchEvmTx)
	}

	txResp, err := ToResponseEvmTx(&tx)
	if err != nil {
		logger.Error(ErrFailedToConvertEvmTx, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertEvmTx)
	}

	resp := EvmTxResponse{
		Tx: txResp,
	}
	return c.JSON(resp)
}

func (h *TxHandler) buildBaseEvmTxQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedEvmTx{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}

func (h *TxHandler) NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).SendString("evm routes are not available on this chain")
}
