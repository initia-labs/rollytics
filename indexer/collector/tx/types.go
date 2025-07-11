package tx

import (
	"encoding/json"

	"github.com/initia-labs/rollytics/types"
)

type CacheData struct {
	RestTxs []RestTx
	EvmTxs  []types.EvmTx
}

type QueryRestTxsResponse struct {
	Txs []RestTx `json:"txs"`
}

type RestTx struct {
	Body       json.RawMessage `json:"body"`
	AuthInfo   json.RawMessage `json:"auth_info"`
	Signatures []string        `json:"signatures"`
}

type RestTxBody struct {
	Messages []struct {
		Type string `json:"@type"`
	} `json:"messages"`
}

type QueryEvmTxsResponse struct {
	Result []types.EvmTx `json:"result"`
}

type PrimaryStoreCreatedEvent struct {
	OwnerAddr    string `json:"owner_addr"`
	StoreAddr    string `json:"store_addr"`
	MetadataAddr string `json:"metadata_addr"`
}

type FAEvent struct {
	StoreAddr string `json:"store_addr"`
}

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
	Result []PrestateTracerTx `json:"result"`
}

type PrestateTracerTx struct {
	TxHash string                `json:"txHash"`
	Result PrestateTracerTxState `json:"result"`
}

type PrestateTracerTxState struct {
	Post map[string]interface{} `json:"post"`
	Pre  map[string]interface{} `json:"pre"`
}