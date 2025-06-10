package tx

import (
	"encoding/json"
	"fmt"

	cbjson "github.com/cometbft/cometbft/libs/json"
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

type TxsRequestByHeight struct {
	Pagination *common.PaginationParams `query:"pagination"`
	Height     int64                    `param:"height"`
}

// TxsByAccountRequest
type TxsByAccountRequest struct {
	Account    string                   `param:"account"`
	Pagination *common.PaginationParams `query:"pagination"`
}

// TxsByHeightRequest
type TxsByHeightRequest struct {
	Height     int64                    `param:"height"`
	Pagination *common.PaginationParams `query:"pagination"`
}

// TxByHashRequest
type TxByHashRequest struct {
	Hash string `param:"tx_hash"`
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
