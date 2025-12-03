package tx

import (
	"encoding/json"

	"github.com/initia-labs/rollytics/types"
)

type CacheData struct {
	RestTxs []types.RestTx
	EvmTxs  []types.EvmTx
}

type QueryRestTxsResponse struct {
	Txs        []RestTx   `json:"txs"`
	Pagination Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	NextKey []byte `json:"next_key"`
	Total   string `json:"total"`
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
