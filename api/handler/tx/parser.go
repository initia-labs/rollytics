package tx

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/codec"
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

	heightStr := c.Params("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height param: "+err.Error())
	}

	return &TxsRequestByHeight{
		Pagination: pagination,
		Height:     height,
	}, nil
}

func ParseTxsByAccountRequest(c *fiber.Ctx) (*TxsByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	account := c.Params("account")
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}

	accAddr, err := codec.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account: "+err.Error())
	}

	return &TxsByAccountRequest{
		Pagination: pagination,
		Account:    accAddr.String(),
	}, nil
}

// ParseTxsByHeightRequest parses and validates the request
func ParseTxsByHeightRequest(c *fiber.Ctx) (*TxsByHeightRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	heightStr := c.Params("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid height param: "+err.Error())
	}

	return &TxsByHeightRequest{
		Pagination: pagination,
		Height:     height,
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

	accAddr, err := codec.AccAddressFromString(account)
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
