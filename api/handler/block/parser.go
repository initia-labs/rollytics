package block

import (
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
)

func ParseBlocksRequest(c *fiber.Ctx) *BlocksRequest {
	pagination := common.ExtractPaginationParams(c)

	return &BlocksRequest{
		Pagination: pagination,
	}
}

func ParseBlockByHeightRequest(c *fiber.Ctx) (*BlockByHeightRequest, error) {
	height, err := common.GetHeightParam(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return &BlockByHeightRequest{
		Height: height,
	}, nil
}
