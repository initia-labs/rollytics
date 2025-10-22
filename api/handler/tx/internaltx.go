package tx

import (
	"database/sql"
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// GetEvmInternalTxs handles GET /tx/v1/evm-internal-txs
// @Summary Get EVM internal transactions
// @Description Get a list of EVM internal transactions with pagination
// @Tags EVM Internal Tx
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-internal-txs [get]
func (h *TxHandler) GetEvmInternalTxs(c *fiber.Ctx) error {
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Use read-only transaction for better performance
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	// Use optimized COUNT - no filters
	query := tx.Model(&types.CollectedEvmInternalTx{})
	var strategy types.CollectedEvmInternalTx
	hasFilters := false // no filters in basic GetEvmInternalTxs
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmInternalTx
	findQuery := pagination.ApplySequence(tx.Model(&types.CollectedEvmInternalTx{}))
	if err := findQuery.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Get accounts for internal txs
	accounts, err := h.getAccounts(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	hashes, err := h.getHashes(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes := ToEvmInternalTxsResponse(txs, accounts, hashes)

	var lastRecord any
	if len(txs) > 0 {
		lastRecord = txs[len(txs)-1]
	}

	return c.JSON(EvmInternalTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponseWithLastRecord(total, lastRecord),
	})
}

// GetEvmInternalTxsByAccount handles GET /tx/v1/evm-internal-txs/by_account/{account}
// @Summary Get EVM internal transactions by account
// @Description Get EVM internal transactions associated with a specific account
// @Tags EVM Internal Tx
// @Accept json
// @Produce json
// @Param account path string true "Account address"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-internal-txs/by_account/{account} [get]
func (h *TxHandler) GetEvmInternalTxsByAccount(c *fiber.Ctx) error {
	account, err := common.GetAccountParam(c)
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

	accountIds, err := h.GetAccountIds([]string{account})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if len(accountIds) == 0 {
		return c.JSON(EvmInternalTxsResponse{
			Txs:        []EvmInternalTxResponse{},
			Pagination: pagination.ToResponse(0),
		})
	}

	query, total, err := buildEvmInternalTxEdgeQuery(tx, accountIds[0], pagination)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmInternalTx
	if err := query.Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Get accounts for internal txs
	accounts, err := h.getAccounts(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	hashes, err := h.getHashes(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes := ToEvmInternalTxsResponse(txs, accounts, hashes)

	return c.JSON(EvmInternalTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
	})
}

func buildEvmInternalTxEdgeQuery(tx *gorm.DB, accountID int64, pagination *common.Pagination) (*gorm.DB, int64, error) {
	sequenceQuery := tx.
		Model(&types.CollectedEvmInternalTxAccount{}).
		Select("sequence").
		Where("account_id = ?", accountID)

	sequenceQuery = sequenceQuery.Distinct("sequence")
	countQuery := sequenceQuery.Session(&gorm.Session{})

	total, err := common.GetCountWithTimeout(countQuery)
	if err != nil {
		return nil, 0, err
	}

	// apply pagination to the sequence query
	sequenceQuery = pagination.ApplySequence(sequenceQuery)

	query := tx.Model(&types.CollectedEvmInternalTx{}).
		Where("sequence IN (?)", sequenceQuery).
		Order(pagination.OrderBy("sequence"))

	return query, total, nil
}

// GetEvmInternalTxsByHeight handles GET /tx/v1/evm-internal-txs/by_height/{height}
// @Summary Get EVM internal transactions by height
// @Description Get EVM internal transactions at a specific block height
// @Tags EVM Internal Tx
// @Accept json
// @Produce json
// @Param height path int true "Block height"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-internal-txs/by_height/{height} [get]
func (h *TxHandler) GetEvmInternalTxsByHeight(c *fiber.Ctx) error {
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

	query := tx.Model(&types.CollectedEvmInternalTx{}).Where("height = ?", height)

	// Use optimized COUNT - always has filters (height)
	var strategy types.CollectedEvmInternalTx
	hasFilters := true // always has height filter
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmInternalTx
	if err := query.
		Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Get accounts for internal txs
	accounts, err := h.getAccounts(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	hashes, err := h.getHashes(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes := ToEvmInternalTxsResponse(txs, accounts, hashes)

	return c.JSON(EvmInternalTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
	})
}

// GetEvmInternalTxsByHash handles GET /tx/v1/evm-internal-txs/{tx_hash}
// @Summary Get EVM internal transaction by hash
// @Description Get a specific EVM internal transaction by its hash
// @Tags EVM Internal Tx
// @Accept json
// @Produce json
// @Param tx_hash path string true "Transaction hash"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Router /indexer/tx/v1/evm-internal-txs/{tx_hash} [get]
func (h *TxHandler) GetEvmInternalTxsByHash(c *fiber.Ctx) error {
	hash, err := common.GetParams(c, "tx_hash")
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

	hashBytes, err := util.HexToBytes(hash)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid hash format")
	}
	var hashDict types.CollectedEvmTxHashDict
	if err := tx.Where("hash = ?", hashBytes).First(&hashDict).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "transaction not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	query := tx.Model(&types.CollectedEvmInternalTx{}).Where("hash_id = ?", hashDict.Id)

	// Use optimized COUNT - always has filters (hash_id)
	var strategy types.CollectedEvmInternalTx
	hasFilters := true // always has hash_id filter
	var total int64
	total, err = common.GetOptimizedCount(query, strategy, hasFilters)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var txs []types.CollectedEvmInternalTx
	if err := query.Order(pagination.OrderBy("sequence")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&txs).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "tx not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Get accounts and hashes for internal txs
	accounts, err := h.getAccounts(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	hashes, err := h.getHashes(tx, txs)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	txsRes := ToEvmInternalTxsResponse(txs, accounts, hashes)

	return c.JSON(EvmInternalTxsResponse{
		Txs:        txsRes,
		Pagination: pagination.ToResponse(total),
	})
}
