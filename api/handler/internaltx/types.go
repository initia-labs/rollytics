package internaltx

import (
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

// Evm Internal Tx
type EvmInternalTxResponse struct {
	Height  int64  `json:"height"`
	Hash    string `json:"hash"`
	Index   int64  `json:"index"`
	Type    string `json:"type"`
	From    string `json:"from"`
	To      string `json:"to"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Value   string `json:"value"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
}

type EvmInternalTxsResponse struct {
	Txs        []EvmInternalTxResponse   `json:"internal_txs" extensions:"x-order:0"`
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

func ToEvmInternalTxsResponse(citxs []types.CollectedEvmInternalTx) ([]EvmInternalTxResponse, error) {
	txs := make([]EvmInternalTxResponse, 0, len(citxs))
	for _, ctx := range citxs {
		tx, err := ToEvmInternalTxResponse(&ctx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *tx)
	}
	return txs, nil
}

func ToEvmInternalTxResponse(citx *types.CollectedEvmInternalTx) (*EvmInternalTxResponse, error) {
	return &EvmInternalTxResponse{
		Height:  citx.Height,
		Hash:    citx.Hash,
		Index:   citx.Index,
		From:    citx.From,
		To:      citx.To,
		Value:   citx.Value,
		Gas:     citx.Gas,
		GasUsed: citx.GasUsed,
		Type:    citx.Type,
		Input:   citx.Input,
		Output:  citx.Output,
	}, nil
}
