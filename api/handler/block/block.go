package block

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	dbtypes "github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

type BlockHandler struct {
	*common.Handler
}

// GetBlocks handles GET /block/v1/blocks
// @Summary Get blocks
// @Description Get a list of blocks with pagination
// @Tags Blocks
// @Accept json
// @Produce json
// @Param pagination.key query string false "Pagination key"
// @Param pagination.offset query int false "Pagination offset"
// @Param pagination.limit query int false "Pagination limit" default(100)
// @Param pagination.count_total query bool false "Count total"
// @Param pagination.reverse query bool false "Reverse order"
// @Router /indexer/block/v1/blocks [get]
func (h *BlockHandler) GetBlocks(c *fiber.Ctx) error {
	req, err := ParseBlocksRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	query := h.buildBaseBlockQuery()
	query, err = req.Pagination.ApplyPagination(query, "height")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	var blocks []dbtypes.CollectedBlock
	if err := query.Find(&blocks).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchBlock)
	}

	blocksResp, err := BatchToResponseBlocks(blocks)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertBlock)
	}

	var nextKey int64
	if len(blocks) > 0 {
		nextKey = blocks[len(blocks)-1].Height
	}

	pageResp, err := req.Pagination.GetPageResponse(len(blocks), h.buildBaseBlockQuery(), nextKey)
	if err != nil {
		return err
	}

	resp := BlocksResponse{
		Blocks:     blocksResp,
		Pagination: pageResp,
	}

	return c.JSON(resp)
}

// GetBlockByHeight handles GET /block/v1/blocks/{height}
// @Summary Get block by height
// @Description Get a specific block by its height
// @Tags Blocks
// @Accept json
// @Produce json
// @Param height path string true "Block height"
// @Router /indexer/block/v1/blocks/{height} [get]
func (h *BlockHandler) GetBlockByHeight(c *fiber.Ctx) error {
	req, err := ParseBlockByHeightRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	height, err := strconv.ParseInt(req.Height, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid height format")
	}

	var block dbtypes.CollectedBlock
	if err := h.buildBaseBlockQuery().Where("height = ?", height).First(&block).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchBlock)
	}

	blockResp, err := ToResponseBlock(&block)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToConvertBlock)
	}

	resp := BlockResponse{
		Block: blockResp,
	}

	return c.JSON(resp)
}

// GetAvgBlockTime handles GET /block/v1/avg_blocktime
// @Summary Get average block time
// @Description Get the average block time over recent blocks
// @Tags Blocks
// @Accept json
// @Produce json
// @Router /indexer/block/v1/avg_blocktime [get]
func (h *BlockHandler) GetAvgBlockTime(c *fiber.Ctx) error {
	var blocks []dbtypes.CollectedBlock
	if err := h.buildBaseBlockQuery().Order("height DESC").Limit(100).Find(&blocks).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, ErrFailedToFetchBlock)
	}

	if len(blocks) < 2 {
		return c.JSON(AvgBlockTimeResponse{AvgBlockTime: 0})
	}

	var totalTime time.Duration
	for i := 0; i < len(blocks)-1; i++ {
		timeDiff := blocks[i].Timestamp.Sub(blocks[i+1].Timestamp)
		totalTime += timeDiff
	}

	avgTime := totalTime.Seconds() / float64(len(blocks)-1)

	resp := AvgBlockTimeResponse{
		AvgBlockTime: avgTime,
	}

	return c.JSON(resp)
}

func (h *BlockHandler) buildBaseBlockQuery() *gorm.DB {
	return h.Model(&dbtypes.CollectedBlock{}).
		Where("chain_id = ?", h.GetChainConfig().ChainId)
}
