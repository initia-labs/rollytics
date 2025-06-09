package tx

import (
	"encoding/json"
	"fmt"
	"strconv"

	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	evmtypes "github.com/initia-labs/rollytics/indexer/collector/tx"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
)

// Txs
// Request
// TxsRequest
type TxsRequest struct {
	Pagination *common.PaginationParams `query:"pagination"`
}

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

type TxsRequestByHeight struct {
	Pagination *common.PaginationParams `query:"pagination"`
	Height     int64                    `param:"height"`
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

// TxsByAccountRequest
type TxsByAccountRequest struct {
	Account    string                   `param:"account"`
	Pagination *common.PaginationParams `query:"pagination"`
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
	req.Account = account

	return req, nil
}

// TxsByHeightRequest
type TxsByHeightRequest struct {
	Height     int64                    `param:"height"`
	Pagination *common.PaginationParams `query:"pagination"`
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

// TxByHashRequest
type TxByHashRequest struct {
	Hash string `param:"tx_hash"`
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

type TxsCountRequest struct{}

// Response
type TxResponse struct {
	Tx *indexertypes.TxByHeightRecord `json:"tx"`
}

type TxsResponse struct {
	Txs        []indexertypes.TxByHeightRecord `json:"txs" extensions:"x-order:0"`
	Pagination *common.PageResponse            `json:"pagination" extensions:"x-order:1"`
}

type TxCountResponse struct {
	Count uint64 `json:"count" extensions:"x-order:0"`
}

type AccountTxResponse struct {
	Txs        []indexertypes.TxByHeightRecord `json:"txs" extensions:"x-order:0"`
	Pagination *common.PageResponse            `json:"pagination" extensions:"x-order:1"`
}

// Conversion functions
func ToResponseTx(ctx *types.CollectedTx) (*indexertypes.TxByHeightRecord, error) {
	var record indexertypes.TxByHeightRecord
	if err := cbjson.Unmarshal(ctx.Data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TxByHeightRecord: %w", err)
	}

	return &record, nil
}

func BatchToResponseTxs(ctxs []types.CollectedTx) ([]indexertypes.TxByHeightRecord, error) {
	txs := make([]indexertypes.TxByHeightRecord, 0, len(ctxs))
	for _, ctx := range ctxs {
		tx, err := ToResponseTx(&ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *tx)
	}
	return txs, nil
}

// EvmTxs
// Request
// TxsRequest
type EvmTxsRequest struct {
	Pagination *common.PaginationParams `query:"pagination"`
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

// EvmTxsByAccountRequest
type EvmTxsByAccountRequest struct {
	Account    string                   `param:"account"`
	Pagination *common.PaginationParams `query:"pagination"`
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
	req.Account = account

	return req, nil
}

// EvmTxsByHeightRequest
type EvmTxsByHeightRequest struct {
	Height     int64                    `param:"height"`
	Pagination *common.PaginationParams `query:"pagination"`
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

// EvmTxByHashRequest
type EvmTxByHashRequest struct {
	Hash string `param:"tx_hash"`
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

type EvmTxsCountRequest struct{}

// Response
type EvmTxResponse struct {
	Tx *evmtypes.EvmTx `json:"tx"`
}

type EvmTxsResponse struct {
	Pagination *common.PageResponse `json:"pagination" extensions:"x-order:0"`
	Txs        []evmtypes.EvmTx     `json:"txs" extensions:"x-order:1"`
}

type EvmTxCountResponse struct {
	Count uint64 `json:"count" extensions:"x-order:0"`
}

func ToResponseEvmTx(ctx *types.CollectedEvmTx) (*evmtypes.EvmTx, error) {
	var evmTx evmtypes.EvmTx
	if err := json.Unmarshal(ctx.Data, &evmTx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evm tx data: %w", err)
	}
	return &evmTx, nil
}

func BatchToResponseEvmTxs(ctxs []types.CollectedEvmTx) ([]evmtypes.EvmTx, error) {
	txs := make([]evmtypes.EvmTx, 0, len(ctxs))
	for _, ctx := range ctxs {
		tx, err := ToResponseEvmTx(&ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *tx)
	}
	return txs, nil
}
