package tx

import (
	"encoding/json"
	"fmt"

	cbjson "github.com/cometbft/cometbft/libs/json"

	"github.com/initia-labs/rollytics/api/handler/common"
	types "github.com/initia-labs/rollytics/types"
)

// Txs
// Request
// TxsRequest
type TxsRequest struct {
	Pagination *common.PaginationParams `query:"pagination"`
	Msgs       []string                 `query:"msgs"` // optional, filter by message types
}

// TxsByHeightRequest
type TxsByHeightRequest struct {
	Height     int64                    `param:"height"`
	Pagination *common.PaginationParams `query:"pagination"`
	Msgs       []string                 `query:"msgs"` // optional, filter by message types
}

// TxsByAccountRequest
type TxsByAccountRequest struct {
	Account    string                   `param:"account"`
	Pagination *common.PaginationParams `query:"pagination"`
	Msgs       []string                 `query:"msgs"` // optional, filter by message types
}

// TxByHashRequest
type TxByHashRequest struct {
	Hash string `param:"tx_hash"`
}

// Response
type TxResponse struct {
	Tx *types.Tx `json:"tx"`
}

type TxsResponse struct {
	Txs        []types.Tx           `json:"txs" extensions:"x-order:0"`
	Pagination *common.PageResponse `json:"pagination" extensions:"x-order:1"`
}

type AccountTxResponse struct {
	Txs        []types.Tx           `json:"txs" extensions:"x-order:0"`
	Pagination *common.PageResponse `json:"pagination" extensions:"x-order:1"`
}

// Conversion functions
func ToResponseTx(ctx *types.CollectedTx) (*types.Tx, error) {
	var record types.Tx
	if err := cbjson.Unmarshal(ctx.Data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Tx: %w", err)
	}

	return &record, nil
}

func BatchToResponseTxs(ctxs []types.CollectedTx) ([]types.Tx, error) {
	txs := make([]types.Tx, 0, len(ctxs))
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

// EvmTxsByAccountRequest
type EvmTxsByAccountRequest struct {
	Account    string                   `param:"account"`
	Pagination *common.PaginationParams `query:"pagination"`
}

// EvmTxsByHeightRequest
type EvmTxsByHeightRequest struct {
	Height     int64                    `param:"height"`
	Pagination *common.PaginationParams `query:"pagination"`
}

// EvmTxByHashRequest
type EvmTxByHashRequest struct {
	Hash string `param:"tx_hash"`
}

// Response
type EvmTxResponse struct {
	Tx *types.EvmTx `json:"tx"`
}

type EvmTxsResponse struct {
	Pagination *common.PageResponse `json:"pagination" extensions:"x-order:0"`
	Txs        []types.EvmTx        `json:"txs" extensions:"x-order:1"`
}

func ToResponseEvmTx(ctx *types.CollectedEvmTx) (*types.EvmTx, error) {
	var evmTx types.EvmTx
	if err := json.Unmarshal(ctx.Data, &evmTx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evm tx data: %w", err)
	}
	return &evmTx, nil
}

func BatchToResponseEvmTxs(ctxs []types.CollectedEvmTx) ([]types.EvmTx, error) {
	txs := make([]types.EvmTx, 0, len(ctxs))
	for _, ctx := range ctxs {
		tx, err := ToResponseEvmTx(&ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *tx)
	}
	return txs, nil
}
