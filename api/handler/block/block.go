package block

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
)

// GetBlocks handles GET /block/v1/blocks
// @Summary Get blocks
// @Description Get a list of blocks with pagination
// @Tags Block
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit, default is 100"
// @Param pagination.count_total query bool false "Count total, default is true"
// @Param pagination.reverse query bool false "Reverse order, default is true. if set to true, the results will be ordered in descending order"
// @Router /indexer/block/v1/blocks [get]
func (h *BlockHandler) GetBlocks(c *fiber.Ctx) error {
	var err error
	req := ParseBlocksRequest(c)

	query := h.buildBaseBlockQuery()
	query, err = req.Pagination.Apply(query, "height")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var blocks []dbtypes.CollectedBlock
	if err := query.Find(&blocks).Error; err != nil {
		h.GetLogger().Error("GetBlocks", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	blocksResp, err := BatchToResponseBlocks(blocks, h.GetChainConfig().RestUrl)
	if err != nil {
		h.GetLogger().Error("GetBlocks", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	pageResp := common.GetPageResponse(req.Pagination, blocks, func(block dbtypes.CollectedBlock) []any {
		return []any{block.Height}
	}, func() int64 {
		var total int64
		if err := query.Count(&total).Error; err != nil {
			return 0
		}
		return total
	})

	return c.JSON(BlocksResponse{
		Blocks:     blocksResp,
		Pagination: pageResp,
	})
}

// GetBlockByHeight handles GET /block/v1/blocks/{height}
// @Summary Get block by height
// @Description Get a specific block by its height
// @Tags Block
// @Accept json
// @Produce json
// @Param height path string true "Block height"
// @Router /indexer/block/v1/blocks/{height} [get]
func (h *BlockHandler) GetBlockByHeight(c *fiber.Ctx) error {
	req, err := ParseBlockByHeightRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var block dbtypes.CollectedBlock
	if err := h.buildBaseBlockQuery().Where("height = ?", req.Height).First(&block).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Block not found")
		}
		h.GetLogger().Error("GetBlockByHeight", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	blockResp, err := ToResponseBlock(&block, h.GetChainConfig().RestUrl)
	if err != nil {
		h.GetLogger().Error("GetBlockByHeight", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(BlockResponse{
		Block: blockResp,
	})
}

// GetAvgBlockTime handles GET /block/v1/avg_blocktime
// @Summary Get average block time
// @Description Get the average block time over recent blocks
// @Tags Block
// @Accept json
// @Produce json
// @Router /indexer/block/v1/avg_blocktime [get]
func (h *BlockHandler) GetAvgBlockTime(c *fiber.Ctx) error {
	var blocks []dbtypes.CollectedBlock
	if err := h.buildBaseBlockQuery().
		Order("height DESC").
		Limit(100).
		Find(&blocks).Error; err != nil {
		h.GetLogger().Error("GetAvgBlockTime", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if len(blocks) < 2 {
		return c.JSON(AvgBlockTimeResponse{
			AvgBlockTime: 0,
		})
	}

	// Calculate average block time
	startTime := blocks[len(blocks)-1].Timestamp
	endTime := blocks[0].Timestamp
	totalTime := endTime.Sub(startTime)
	avgTime := totalTime.Seconds() / float64(len(blocks)-1)

	return c.JSON(AvgBlockTimeResponse{
		AvgBlockTime: avgTime,
	})
}

func (h *BlockHandler) buildBaseBlockQuery() *gorm.DB {
	return h.GetDatabase().Model(&dbtypes.CollectedBlock{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
