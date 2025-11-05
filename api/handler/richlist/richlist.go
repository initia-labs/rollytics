package richlist

import (
	"database/sql"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func (h *RichListHandler) GetTokenHolders(c *fiber.Ctx) error {
	denom := c.Params("denom")
	if denom == "" {
		return fiber.NewError(fiber.StatusBadRequest, "denom parameter is required")
	}

	denom = strings.ToLower(denom)

	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	// Start read-only transaction
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	// Query rich list ordered by amount DESC with pagination
	var richListRecords []types.CollectedRichList
	if err := tx.Model(&types.CollectedRichList{}).
		Where("denom = ?", denom).
		Order("amount DESC").
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
