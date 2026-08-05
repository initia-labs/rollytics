package richlist

import (
	"database/sql"
	"net/url"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

// GetTokenHolders handles GET /indexer/richlist/v1/*
// @Summary Get token holders
// @Description Get a list of token holders for a specific denomination, ordered by amount in descending order. Denoms may contain '/' (e.g. move/..., ibc/...) or spaces (URL-encode as %20).
// @Tags Rich List
// @Accept json
// @Produce json
// @Param denom path string true "Token denomination (supports '/', encode spaces as %20)"
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100" default is 100
// @Param pagination.count_total query bool false "Count total, default is true" default is true
// @Param pagination.reverse query bool false "Reverse order default is true if set to true, the results will be ordered in descending order"
// @Success 200 {object} TokenHoldersResponse
// @Router /indexer/richlist/v1/{denom} [get]
func (h *RichListHandler) GetTokenHolders(c *fiber.Ctx) error {
	denom, err := parseDenomFromPath(c)
	if err != nil {
		return err
	}
	if h.cfg.GetVmType() == types.EVM {
		contract, err := h.querier.GetEvmContractByDenom(c.Context(), denom)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		denom = contract
	}

	pagination, err := common.ParsePagination(c, common.CursorTypeOffset)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Start read-only transaction
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	// Query rich list ordered by amount with pagination
	var richListRecords []types.CollectedRichList
	if err := tx.Model(&types.CollectedRichList{}).
		Where("denom = ?", denom).
		Order(pagination.OrderBy("amount")).
		Limit(pagination.Limit).
		Offset(pagination.Offset).
		Find(&richListRecords).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to fetch token holders")
	}

	// Extract unique account IDs
	accountIds := make([]int64, len(richListRecords))
	for i, record := range richListRecords {
		accountIds[i] = record.Id
	}

	// Fetch account addresses in a single query
	var accounts []types.CollectedAccountDict
	if len(accountIds) > 0 {
		if err := tx.Table("account_dict").
			Select("id, account").
			Where("id IN ?", accountIds).
			Find(&accounts).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to fetch account addresses")
		}
	}

	// Create account ID to address mapping
	accountMap := make(map[int64]string, len(accounts))
	for _, acc := range accounts {
		if h.cfg.GetVmType() == types.EVM {
			accountMap[acc.Id] = util.BytesToHexWithPrefix(acc.Account)
		} else {
			accountMap[acc.Id] = sdk.AccAddress(acc.Account).String()
		}
	}

	// Map results to response format
	holders := make([]TokenHolder, len(richListRecords))
	for i, record := range richListRecords {
		holders[i] = TokenHolder{
			Account: accountMap[record.Id],
			Amount:  record.Amount,
		}
	}

	// Count total if requested
	var total int64
	if pagination.CountTotal {
		countQuery := tx.Model(&types.CollectedRichList{}).
			Where("denom = ?", denom)
		if err := countQuery.Count(&total).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to count total holders")
		}
	}

	// Build pagination response
	hasMore := len(richListRecords) == pagination.Limit
	paginationResp := pagination.ToResponse(total, hasMore)

	return c.JSON(TokenHoldersResponse{
		Holders:    holders,
		Pagination: paginationResp,
	})
}

// parseDenomFromPath extracts and normalizes the denom from the wildcard path.
// Supports raw slashes (move/..., ibc/...) and percent-encoding (%20, %2F).
func parseDenomFromPath(c *fiber.Ctx) (string, error) {
	raw := c.Params("*")
	if raw == "" {
		// Fallback for older :denom route style if ever remounted.
		raw = c.Params("denom")
	}
	raw = strings.TrimPrefix(raw, "/")
	if raw == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "denom parameter is required")
	}

	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", fiber.NewError(fiber.StatusBadRequest, "invalid denom encoding")
	}
	return strings.ToLower(decoded), nil
}
