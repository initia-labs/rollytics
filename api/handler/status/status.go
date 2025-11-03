package status

import (
	"database/sql"
	"errors"
	"sync/atomic"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
)

var (
	lastEvmInternalTxHeight atomic.Int64
	lastRichListHeight      atomic.Int64
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
		lastBlock          types.CollectedBlock
		lastEvmInternalTx  types.CollectedEvmInternalTx
		lastRichListStatus types.CollectedRichListStatus
	)

	// Use single transaction for consistent snapshot
	tx := h.GetDatabase().Begin(&sql.TxOptions{ReadOnly: true})
	defer tx.Rollback()

	if err := tx.
		Model(&types.CollectedBlock{}).
		Where("chain_id = ?", h.GetChainId()).
		Order("height DESC").
		First(&lastBlock).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	internalTxHeight := lastEvmInternalTxHeight.Load()
	//nolint:nestif
	if h.isInternalTxEnabledEvm() {
		if err := tx.
			Model(&types.CollectedEvmInternalTx{}).
			Order("height DESC").
			First(&lastEvmInternalTx).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fiber.NewError(fiber.StatusInternalServerError, err.Error())
			}
		}

		// replace if current height is higher than last one using compare-and-swap
		if lastEvmInternalTx.Height > 0 && lastEvmInternalTx.Height > internalTxHeight {
			if lastEvmInternalTxHeight.CompareAndSwap(internalTxHeight, lastEvmInternalTx.Height) {
				internalTxHeight = lastEvmInternalTx.Height
			} else {
				internalTxHeight = lastEvmInternalTxHeight.Load()
			}
		}

		var exists bool
		if err := tx.
			Model(&types.CollectedBlock{}).
			Select("1").
			Where("chain_id = ?", h.GetChainId()).
			Where("height > ?", internalTxHeight).
			Where("tx_count > 0").
			Limit(1).
			Find(&exists).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		if !exists {
			internalTxHeight = lastBlock.Height
		}
	}

	richListHeight := lastRichListHeight.Load()
	if h.isRichListEnabled() {
		if err := tx.
			Model(&types.CollectedRichListStatus{}).
			Order("height DESC").
			First(&lastRichListStatus).Error; err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		if lastRichListStatus.Height > 0 && lastRichListStatus.Height > richListHeight {
			if lastRichListHeight.CompareAndSwap(richListHeight, lastRichListStatus.Height) {
				richListHeight = lastRichListStatus.Height
			} else {
				richListHeight = lastRichListHeight.Load()
			}
		}
	}

	return c.JSON(&StatusResponse{
		Version:          config.Version,
		CommitHash:       config.CommitHash,
		ChainId:          h.GetChainId(),
		Height:           lastBlock.Height,
		InternalTxHeight: internalTxHeight,
		RichListHeight:   richListHeight,
	})
}

func (h *StatusHandler) isInternalTxEnabledEvm() bool {
	return (h.GetChainConfig().VmType == types.EVM) && h.GetConfig().InternalTxEnabled()
}

func (h *StatusHandler) isRichListEnabled() bool {
	return h.GetConfig().GetRichListEnabled()
}
