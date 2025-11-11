package block

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/common-handler/common"
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
// @Param pagination.reverse query bool false "Reverse order, default is true. if set to true, the results will be ordered in descending order"
// @Router /indexer/block/v1/blocks [get]
func (h *BlockHandler) GetBlocks(c *fiber.Ctx) error {
	pagination, err := common.ParsePagination(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var lastBlock types.CollectedBlock
	if err := h.buildBaseBlockQuery().
		Order("height DESC").
		Limit(1).
		First(&lastBlock).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	total := lastBlock.Height

	var blocks []types.CollectedBlock
	if err := h.buildBaseBlockQuery().
		Order(pagination.OrderBy("height")).
		Offset(pagination.Offset).
		Limit(pagination.Limit).
		Find(&blocks).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	blocksRes, err := ToBlocksResponse(blocks, h.GetConfig())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(BlocksResponse{
		Blocks:     blocksRes,
		Pagination: pagination.ToResponse(total, len(blocks) == pagination.Limit),
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
	height, err := common.GetHeightParam(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var block types.CollectedBlock
	if err := h.buildBaseBlockQuery().
		Where("height = ?", height).
		First(&block).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(fiber.StatusNotFound, types.NewNotFoundError("block").Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, types.NewDatabaseError("get block", err).Error())
	}

	blockRes, err := ToBlockResponse(block, h.GetConfig())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(BlockResponse{
		Block: blockRes,
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
	var cbs []types.CollectedBlock
	if err := h.buildBaseBlockQuery().
		Order("height DESC").
		Limit(100).
		Find(&cbs).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if len(cbs) < 2 {
		return c.JSON(AvgBlockTimeResponse{
			AvgBlockTime: 0,
		})
	}

	startTime := cbs[len(cbs)-1].Timestamp
	endTime := cbs[0].Timestamp
	totalTime := endTime.Sub(startTime)
	avgTime := totalTime.Seconds() / float64(len(cbs)-1)

	return c.JSON(AvgBlockTimeResponse{
		AvgBlockTime: avgTime,
	})
}

func (h *BlockHandler) buildBaseBlockQuery() *gorm.DB {
	return h.GetDatabase().
		Model(&types.CollectedBlock{}).
		Where("chain_id = ?", h.GetChainId())
}
