package tx

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
)

func ParseTxsRequest(c *fiber.Ctx) (*TxsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &TxsRequest{
		Pagination: pagination,
	}

	return req, nil
}

func ParseTxsRequestByHeight(c *fiber.Ctx) (*TxsRequestByHeight, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &TxsRequestByHeight{
		Pagination: pagination,
	}

	heightStr := c.Params("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height param")
	}
	req.Height = height

	return req, nil
}

func ParseTxsByAccountRequest(c *fiber.Ctx) (*TxsByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &TxsByAccountRequest{
		Pagination: pagination,
	}

	account := c.Params("account")
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}

	accAddr, err := common.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account")
	}

	req.Account = accAddr.String()
	return req, nil
}

// ParseTxsByHeightRequest parses and validates the request
func ParseTxsByHeightRequest(c *fiber.Ctx) (*TxsByHeightRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &TxsByHeightRequest{
		Pagination: pagination,
	}

	heightStr := c.Params("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height param")
	}
	req.Height = height

	return req, nil
}

func ParseTxByHashRequest(c *fiber.Ctx) (*TxByHashRequest, error) {
	req := &TxByHashRequest{}

	hash := c.Params("tx_hash")
	if hash == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid tx_hash param")
	}
	req.Hash = hash

	return req, nil
}

func ParseEvmTxsRequest(c *fiber.Ctx) (*EvmTxsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &EvmTxsRequest{
		Pagination: pagination,
	}

	return req, nil
}

func ParseEvmTxsByAccountRequest(c *fiber.Ctx) (*EvmTxsByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}
	req := &EvmTxsByAccountRequest{
		Pagination: pagination,
	}

	account := c.Params("account")
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}

	accAddr, err := common.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account")
	}
	req.Account = accAddr.String()

	return req, nil
}

// ParseEvmTxsByHeightRequest parses and validates the request
func ParseEvmTxsByHeightRequest(c *fiber.Ctx) (*EvmTxsByHeightRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &EvmTxsByHeightRequest{
		Pagination: pagination,
	}

	heightStr := c.Params("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height param")
	}
	req.Height = height

	return req, nil
}

func ParseEvmTxByHashRequest(c *fiber.Ctx) (*EvmTxByHashRequest, error) {
	req := &EvmTxByHashRequest{}

	hash := c.Params("tx_hash")
	if hash == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid tx_hash param")
	}
	req.Hash = hash

	return req, nil
}
