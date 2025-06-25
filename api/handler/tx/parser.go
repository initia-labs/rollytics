package tx

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/util"
)

func ParseTxsRequest(c *fiber.Ctx) (*TxsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	raw := c.Request().URI().QueryArgs().PeekMulti("msgs")
	msgs := make([]string, len(raw))
	for i, b := range raw {
		msgs[i] = string(b)
	}

	return &TxsRequest{
		Pagination: pagination,
		Msgs:       msgs,
	}, nil
}

func ParseTxsByHeightRequest(c *fiber.Ctx) (*TxsByHeightRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	raw := c.Request().URI().QueryArgs().PeekMulti("msgs")
	msgs := make([]string, len(raw))
	for i, b := range raw {
		msgs[i] = string(b)
	}

	heightStr := c.Params("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height param: "+err.Error())
	}

	return &TxsByHeightRequest{
		Pagination: pagination,
		Height:     height,
		Msgs:       msgs,
	}, nil
}

func ParseTxsByAccountRequest(c *fiber.Ctx) (*TxsByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	raw := c.Request().URI().QueryArgs().PeekMulti("msgs")
	msgs := make([]string, len(raw))
	for i, b := range raw {
		msgs[i] = string(b)
	}

	account := c.Params("account")
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}

	accAddr, err := util.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account: "+err.Error())
	}

	return &TxsByAccountRequest{
		Pagination: pagination,
		Account:    accAddr.String(),
		Msgs:       msgs,
	}, nil
}

func ParseTxByHashRequest(c *fiber.Ctx) (*TxByHashRequest, error) {
	hash := c.Params("tx_hash")
	if hash == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid tx_hash param")
	}

	return &TxByHashRequest{
		Hash: hash,
	}, nil
}

func ParseEvmTxsRequest(c *fiber.Ctx) (*EvmTxsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	return &EvmTxsRequest{
		Pagination: pagination,
	}, nil
}

func ParseEvmTxsByAccountRequest(c *fiber.Ctx) (*EvmTxsByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}
	account := c.Params("account")
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}

	accAddr, err := util.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account: "+err.Error())
	}

	return &EvmTxsByAccountRequest{
		Pagination: pagination,
		Account:    accAddr.String(),
	}, nil
}

// ParseEvmTxsByHeightRequest parses and validates the request
func ParseEvmTxsByHeightRequest(c *fiber.Ctx) (*EvmTxsByHeightRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	heightStr := c.Params("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height param: "+err.Error())
	}

	return &EvmTxsByHeightRequest{
		Pagination: pagination,
		Height:     height,
	}, nil
}

func ParseEvmTxByHashRequest(c *fiber.Ctx) (*EvmTxByHashRequest, error) {
	hash := c.Params("tx_hash")
	if hash == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid tx_hash param: ")
	}

	return &EvmTxByHashRequest{
		Hash: hash,
	}, nil
}
