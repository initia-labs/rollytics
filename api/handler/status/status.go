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

	internalTxHeight := int64(0)
	if h.GetChainConfig().VmType == types.EVM && h.GetConfig().InternalTxEnabled() {
		if err := h.GetDatabase().
			Model(&types.CollectedEvmInternalTx{}).
			Order("height DESC").
			First(&lastEvmInternalTx).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		itxHeight := lastEvmInternalTx.Height
		var count int64
		if err := h.GetDatabase().Model(&types.CollectedBlock{}).
			Where("block.chain_id = ?", h.GetChainId()).
			Where("height > ?", itxHeight).
			Where("tx_count > 0").
			Count(&count).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		if count > 0 {
			internalTxHeight = itxHeight
		} else {
			internalTxHeight = lastBlock.Height
		}
	}

	return c.JSON(&StatusResponse{
		Version:          config.Version,
		CommitHash:       config.CommitHash,
		ChainId:          h.GetChainId(),
		Height:           lastBlock.Height,
		InternalTxHeight: internalTxHeight,
	})
}
