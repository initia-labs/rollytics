package block

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
)

func ParseBlocksRequest(c *fiber.Ctx) (*BlocksRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	return &BlocksRequest{
		Pagination: pagination,
	}, nil
}

func ParseBlockByHeightRequest(c *fiber.Ctx) (*BlockByHeightRequest, error) {
	height := c.Params("height")
	if height == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "height param is required")
	}

	heightInt, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height format: "+err.Error())
	}

	return &BlockByHeightRequest{
		Height: heightInt,
	}, nil
}
