package tx

import (
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
)

func ParseTxsRequest(c *fiber.Ctx) *TxsRequest {
	pagination := common.ExtractPaginationParams(c)
	msgs := common.GetMsgsParams(c)

	return &TxsRequest{
		Pagination: pagination,
		Msgs:       msgs,
	}
}

func ParseTxsByHeightRequest(c *fiber.Ctx) (*TxsByHeightRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	msgs := common.GetMsgsParams(c)
	height, err := common.GetHeightParam(c)
	if err != nil {
		return nil, err
	}

	return &TxsByHeightRequest{
		Pagination: pagination,
		Height:     height,
		Msgs:       msgs,
	}, nil
}

func ParseTxsByAccountRequest(c *fiber.Ctx) (*TxsByAccountRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	msgs := common.GetMsgsParams(c)

	accAddr, err := common.GetAccountParam(c)
	if err != nil {
		return nil, err
	}

	return &TxsByAccountRequest{
		Pagination: pagination,
		Account:    accAddr.String(),
		Msgs:       msgs,
		IsSigner:   c.Query("is_signer", "false") == "true",
	}, nil
}

func ParseTxByHashRequest(c *fiber.Ctx) (*TxByHashRequest, error) {
	hash, err := common.GetParams(c, "tx_hash")
	if err != nil {
		return nil, err
	}

	return &TxByHashRequest{
		Hash: hash,
	}, nil
}

func ParseEvmTxsRequest(c *fiber.Ctx) *EvmTxsRequest {
	pagination := common.ExtractPaginationParams(c)

	return &EvmTxsRequest{
		Pagination: pagination,
	}
}

func ParseEvmTxsByAccountRequest(c *fiber.Ctx) (*EvmTxsByAccountRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	accAddr, err := common.GetAccountParam(c)
	if err != nil {
		return nil, err
	}
	return &EvmTxsByAccountRequest{
		Pagination: pagination,
		Account:    accAddr.String(),
		IsSigner:   c.Query("is_signer", "false") == "true",
	}, nil
}

// ParseEvmTxsByHeightRequest parses and validates the request
func ParseEvmTxsByHeightRequest(c *fiber.Ctx) (*EvmTxsByHeightRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	height, err := common.GetHeightParam(c)
	if err != nil {
		return nil, err
	}

	return &EvmTxsByHeightRequest{
		Pagination: pagination,
		Height:     height,
	}, nil
}

func ParseEvmTxByHashRequest(c *fiber.Ctx) (*EvmTxByHashRequest, error) {
	hash, err := common.GetParams(c, "tx_hash")
	if err != nil {
		return nil, err
	}

	return &EvmTxByHashRequest{
		Hash: hash,
	}, nil
}
