package status

import (
	"database/sql"
	"errors"
	"sync/atomic"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
)

var (
	lastEvmInternalTxHeight atomic.Int64
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

	edgeStatus, err := h.getEdgeBackfillStatus(tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(&StatusResponse{
		Version:          config.Version,
		CommitHash:       config.CommitHash,
		ChainId:          h.GetChainId(),
		Height:           lastBlock.Height,
		InternalTxHeight: internalTxHeight,
		EdgeBackfill:     edgeStatus,
	})
}

func (h *StatusHandler) getEdgeBackfillStatus(tx *gorm.DB) (EdgeBackfillSummary, error) {
	var summary EdgeBackfillSummary

	if details, err := h.toEdgeDetails(tx, types.SeqInfoTxEdgeBackfill); err != nil {
		return summary, err
	} else {
		summary.Tx = details
	}

	if details, err := h.toEdgeDetails(tx, types.SeqInfoEvmTxEdgeBackfill); err != nil {
		return summary, err
	} else {
		summary.EvmTx = details
	}

	if details, err := h.toEdgeDetails(tx, types.SeqInfoEvmInternalTxEdgeBackfill); err != nil {
		return summary, err
	} else {
		summary.EvmInternal = details
	}

	return summary, nil
}

func (h *StatusHandler) toEdgeDetails(tx *gorm.DB, name types.SeqInfoName) (EdgeBackfillDetails, error) {
	status, err := common.GetEdgeBackfillStatus(tx, name)
	if err != nil {
		return EdgeBackfillDetails{}, err
	}

	return EdgeBackfillDetails{
		Completed: status.Completed,
		Sequence:  status.Sequence,
	}, nil
}

func (h *StatusHandler) isInternalTxEnabledEvm() bool {
	return (h.GetChainConfig().VmType == types.EVM) && h.GetConfig().InternalTxEnabled()
}
