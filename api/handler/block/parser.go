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
	req := &BlocksRequest{
		Pagination: pagination,
	}

	return req, nil
}

func ParseBlockByHeightRequest(c *fiber.Ctx) (*BlockByHeightRequest, error) {
	req := &BlockByHeightRequest{
		Height: c.Params("height"),
	}

	if req.Height == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "height param is required")
	}

	if _, err := strconv.ParseInt(req.Height, 10, 64); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height format")
	}

	return req, nil
}
