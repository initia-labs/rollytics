package status

import (
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
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
	var (
		lastBlock         types.CollectedBlock
		lastEvmInternalTx types.CollectedEvmInternalTx
	)

	if err := h.GetDatabase().
		Model(&types.CollectedBlock{}).
		Where("block.chain_id = ?", h.GetChainId()).
		Order("height DESC").
		First(&lastBlock).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if h.GetChainConfig().VmType == types.EVM && h.GetConfig().InternalTxEnabled() {
		if err := h.GetDatabase().
			Model(&types.CollectedEvmInternalTx{}).
			Order("height DESC").
			First(&lastEvmInternalTx).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
	}

	return c.JSON(&StatusResponse{
		Version:          config.Version,
		CommitHash:       config.CommitHash,
		ChainId:          h.GetChainId(),
		Height:           lastBlock.Height,
		InternalTxHeight: lastEvmInternalTx.Height,
	})
}
