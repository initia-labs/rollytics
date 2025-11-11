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
	var lastBlock types.CollectedBlock

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

	var internalTxHeight int64
	if h.isInternalTxEnabledEvm() {
		height, err := h.getInternalTxHeight(tx, lastBlock.Height)
		if err != nil {
			return err
		}
		internalTxHeight = height
	}

	var richListHeight int64
	if h.isRichListEnabled() {
		height, err := h.getRichListHeight(tx)
		if err != nil {
			return err
		}
		richListHeight = height
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

func (h *StatusHandler) getInternalTxHeight(tx *gorm.DB, lastBlockHeight int64) (int64, error) {
	internalTxHeight := lastEvmInternalTxHeight.Load()

	var lastEvmInternalTx types.CollectedEvmInternalTx
	err := tx.Model(&types.CollectedEvmInternalTx{}).Order("height DESC").First(&lastEvmInternalTx).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	// Replace if current height is higher than last one using compare-and-swap
	if lastEvmInternalTx.Height > internalTxHeight {
		if lastEvmInternalTxHeight.CompareAndSwap(internalTxHeight, lastEvmInternalTx.Height) {
			internalTxHeight = lastEvmInternalTx.Height
		} else {
			internalTxHeight = lastEvmInternalTxHeight.Load()
		}
	}

	var exists bool
	err = tx.Model(&types.CollectedBlock{}).
		Select("1").
		Where("chain_id = ?", h.GetChainId()).
		Where("height > ?", internalTxHeight).
		Where("tx_count > 0").
		Limit(1).
		Find(&exists).Error
	if err != nil {
		return 0, fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if !exists {
		internalTxHeight = lastBlockHeight
	}

	return internalTxHeight, nil
}

func (h *StatusHandler) isRichListEnabled() bool {
	return h.GetConfig().GetRichListEnabled()
}

func (h *StatusHandler) getRichListHeight(tx *gorm.DB) (int64, error) {
	richListHeight := lastRichListHeight.Load()

	var lastRichListStatus types.CollectedRichListStatus
	err := tx.Model(&types.CollectedRichListStatus{}).First(&lastRichListStatus).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if lastRichListStatus.Height > richListHeight {
		if lastRichListHeight.CompareAndSwap(richListHeight, lastRichListStatus.Height) {
			return lastRichListStatus.Height, nil
		}
		return lastRichListHeight.Load(), nil
	}

	return richListHeight, nil
}
