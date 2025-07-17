package internal_tx

import (
	"encoding/json"

	"github.com/initia-labs/rollytics/types"
)

type CallTracerResponse struct {
	Result []TracingCall `json:"result"`
}

type TracingCall struct {
	Type    string                `json:"type"`
	From    string                `json:"from"`
	To      string                `json:"to"`
	Value   string                `json:"value"`
	Gas     string                `json:"gas"`
	GasUsed string                `json:"gasUsed"`
	Input   string                `json:"input"`
	Output  string                `json:"output"`
	Calls   []types.EvmInternalTx `json:"calls"`
}

type PrestateTracerResponse struct {
	Result []PrestateTraceResult `json:"result"`
}

type PrestateTraceResult struct {
	TxHash string                `json:"txHash"`
	Result PrestateTracerTxState `json:"result"`
}

type PrestateTracerTxState struct {
	Post json.RawMessage `json:"post"`
	Pre  json.RawMessage `json:"pre"`
}
