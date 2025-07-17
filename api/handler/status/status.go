package status

import (
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/types"
)

// status handles GET /status
// @Summary Status check
// @Description Get current indexer status including chain ID and latest block height
// @Tags App
// @Accept json
// @Produce json
// @Success 200 {object} StatusResponse
// @Router /status [get]
func (h *StatusHandler) GetStatus(c *fiber.Ctx) error {
	cfg := h.GetConfig()
	var lastBlock types.CollectedBlock
	if err := h.GetDatabase().
		Model(&types.CollectedBlock{}).
		Where("block.chain_id = ?", h.GetChainId()).
		Order("height DESC").
		Limit(1).
		First(&lastBlock).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(&StatusResponse{
		Version:    cfg.GetVersion(),
		CommitHash: cfg.GetCommitHash(),
		ChainId:    h.GetChainId(),
		Height:     lastBlock.Height,
	})
}
