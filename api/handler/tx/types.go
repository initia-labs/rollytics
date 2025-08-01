package tx

import (
	"encoding/json"
	"fmt"

	cbjson "github.com/cometbft/cometbft/libs/json"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// Tx
type TxResponse struct {
	Tx types.Tx `json:"tx"`
}

type TxsResponse struct {
	Txs        []types.Tx                `json:"txs" extensions:"x-order:0"`
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

func ToTxsResponse(ctxs []types.CollectedTx) ([]types.Tx, error) {
	txs := make([]types.Tx, 0, len(ctxs))
	for _, ctx := range ctxs {
		tx, err := ToTxResponse(ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func ToTxResponse(ctx types.CollectedTx) (tx types.Tx, err error) {
	if err := cbjson.Unmarshal(ctx.Data, &tx); err != nil {
		return tx, fmt.Errorf("failed to unmarshal Tx: %w", err)
	}

	return tx, nil
}

// Evm Tx
type EvmTxResponse struct {
	Tx types.EvmTx `json:"tx"`
}

type EvmTxsResponse struct {
	Txs        []types.EvmTx             `json:"txs" extensions:"x-order:0"`
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

func ToEvmTxsResponse(ctxs []types.CollectedEvmTx) ([]types.EvmTx, error) {
	txs := make([]types.EvmTx, 0, len(ctxs))
	for _, ctx := range ctxs {
		tx, err := ToEvmTxResponse(ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

func ToEvmTxResponse(ctx types.CollectedEvmTx) (evmTx types.EvmTx, err error) {
	if err := json.Unmarshal(ctx.Data, &evmTx); err != nil {
		return evmTx, fmt.Errorf("failed to unmarshal evm tx data: %w", err)
	}
	return evmTx, nil
}

// Evm Internal Tx
type EvmInternalTxResponse struct {
	Height      int64  `json:"height"`
	Hash        string `json:"hash"`
	ParentIndex int64  `json:"parent_index"`
	Index       int64  `json:"index"`
	Type        string `json:"type"`
	From        string `json:"from"`
	To          string `json:"to"`
	Input       string `json:"input"`
	Output      string `json:"output"`
	Value       string `json:"value"`
	Gas         string `json:"gas"`
	GasUsed     string `json:"gasUsed"`
}

type EvmInternalTxsResponse struct {
	Txs        []EvmInternalTxResponse   `json:"internal_txs" extensions:"x-order:0"`
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

func ToEvmInternalTxsResponse(citxs []types.CollectedEvmInternalTx, accounts map[int64][]byte, txHashs map[int64][]byte) []EvmInternalTxResponse {
	txs := make([]EvmInternalTxResponse, 0, len(citxs))
	for _, ctx := range citxs {
		tx := ToEvmInternalTxResponse(&ctx, accounts, txHashs)
		txs = append(txs, *tx)
	}
	return txs
}

func ToEvmInternalTxResponse(eitx *types.CollectedEvmInternalTx, accounts map[int64][]byte, txHashs map[int64][]byte) *EvmInternalTxResponse {
	var fromAccount, toAccount string

	if eitx.FromId != 0 {
		fromAccount = util.BytesToHexWithPrefix(accounts[eitx.FromId])
	}

	if eitx.ToId != 0 {
		toAccount = util.BytesToHexWithPrefix(accounts[eitx.ToId])
	}

	return &EvmInternalTxResponse{
		Height:      eitx.Height,
		Hash:        util.BytesToHexWithPrefix(txHashs[eitx.HashId]),
		ParentIndex: eitx.ParentIndex,
		Index:       eitx.Index,
		From:        fromAccount,
		To:          toAccount,
		Value:       util.BytesToHexWithPrefix(eitx.Value),
		Gas:         util.BytesToHexWithPrefix(eitx.Gas),
		GasUsed:     util.BytesToHexWithPrefix(eitx.GasUsed),
		Type:        eitx.Type,
		Input:       util.BytesToHexWithPrefix(eitx.Input),
		Output:      util.BytesToHexWithPrefix(eitx.Output),
	}
}
