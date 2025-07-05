package tx

import (
	"encoding/json"
	"fmt"

	cbjson "github.com/cometbft/cometbft/libs/json"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

// Tx
type TxResponse struct {
	Tx *types.Tx `json:"tx"`
}

type TxsResponse struct {
	Txs        []types.Tx                `json:"txs" extensions:"x-order:0"`
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

func ToTxsResponse(ctxs []types.CollectedTx) ([]types.Tx, error) {
	txs := make([]types.Tx, 0, len(ctxs))
	for _, ctx := range ctxs {
		tx, err := ToTxResponse(&ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *tx)
	}
	return txs, nil
}

func ToTxResponse(ctx *types.CollectedTx) (*types.Tx, error) {
	var record types.Tx
	if err := cbjson.Unmarshal(ctx.Data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Tx: %w", err)
	}

	return &record, nil
}

// Evm Tx
type EvmTxResponse struct {
	Tx *types.EvmTx `json:"tx"`
}

type EvmTxsResponse struct {
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:0"`
	Txs        []types.EvmTx             `json:"txs" extensions:"x-order:1"`
}

func ToEvmTxsResponse(ctxs []types.CollectedEvmTx) ([]types.EvmTx, error) {
	txs := make([]types.EvmTx, 0, len(ctxs))
	for _, ctx := range ctxs {
		tx, err := ToEvmTxResponse(&ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *tx)
	}
	return txs, nil
}

func ToEvmTxResponse(ctx *types.CollectedEvmTx) (*types.EvmTx, error) {
	var evmTx types.EvmTx
	if err := json.Unmarshal(ctx.Data, &evmTx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evm tx data: %w", err)
	}
	return &evmTx, nil
}
