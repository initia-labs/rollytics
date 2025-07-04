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
// @Tags EVM Tx
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-txs [get]
func (h *TxHandler) GetEvmTxs(c *fiber.Ctx) (err error) {
	req := ParseEvmTxsRequest(c)
	query := h.buildBaseEvmTxQuery()
	query, err = req.Pagination.Apply(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []dbtypes.CollectedEvmTx
	if err := query.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// response
	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	pageResp := common.GetPageResponse(req.Pagination, txs, func(tx dbtypes.CollectedEvmTx) []any {
		return []any{tx.Sequence}
	}, func() int64 {
		var tx dbtypes.CollectedEvmTx
		if h.buildBaseEvmTxQuery().Select("sequence").Order("sequence DESC").First(&tx).Error != nil {
			return 0
		}
		return int64(tx.Sequence)
	})

	return c.JSON(EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
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
	req, err := ParseEvmTxsByAccountRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.GetDatabase().Model(&dbtypes.CollectedEvmTx{}).
		Select("evm_tx.data", "evm_account_tx.sequence as sequence").
		Joins("INNER JOIN evm_account_tx ON evm_tx.chain_id = evm_account_tx.chain_id AND evm_tx.hash = evm_account_tx.hash").
		Where("evm_account_tx.chain_id = ?", h.GetChainConfig().ChainId).
		Where("evm_account_tx.account = ?", req.Account)

	// If the IsSigner flag is set, filter by signer accounts
	if req.IsSigner {
		query = query.Where("tx.signer = ?", req.Account)
	}

	query, err = req.Pagination.Apply(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []dbtypes.CollectedEvmTx
	if err := query.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// response
	pageResp := common.GetPageResponse(req.Pagination, txs, func(tx dbtypes.CollectedEvmTx) []any {
		return []any{tx.Sequence}
	}, func() int64 {
		var total int64
		if h.GetDatabase().Model(&dbtypes.CollectedEvmAccountTx{}).
			Where("chain_id = ?", h.GetChainConfig().ChainId).
			Where("account = ?", req.Account).Count(&total).Error != nil {
			return 0
		}
		return total
	})
	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
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
	req, err := ParseEvmTxsByHeightRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseEvmTxQuery().
		Where("height = ?", req.Height)
	query, err = req.Pagination.Apply(query, "sequence")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []dbtypes.CollectedEvmTx
	if err := query.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// response
	txsResp, err := BatchToResponseEvmTxs(txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	pageResp := common.GetPageResponse(req.Pagination, txs, func(tx dbtypes.CollectedEvmTx) []any {
		return []any{tx.Sequence}
	}, func() int64 {
		var total int64
		if h.buildBaseEvmTxQuery().Where("height = ?", req.Height).Count(&total).Error != nil {
			return 0
		}
		return total
	})

	return c.JSON(EvmTxsResponse{
		Txs:        txsResp,
		Pagination: pageResp,
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
	req, err := ParseEvmTxByHashRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseEvmTxQuery()
	query = query.Where("hash = ?", req.Hash)

	var tx dbtypes.CollectedEvmTx
	if err := query.Find(&tx).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txResp, err := ToResponseEvmTx(&tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(EvmTxResponse{
		Tx: txResp,
	})
}

func (h *TxHandler) buildBaseEvmTxQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedEvmTx{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}

func (h *TxHandler) NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).SendString("evm routes are not available on this chain")
}
