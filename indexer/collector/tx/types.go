package tx

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/initia-labs/rollytics/types"
)

type QueryEvmTxsResponse struct {
	Result []types.EvmTx `json:"result"`
}

type intoAny interface {
	AsAny() *codectypes.Any
}

type PrimaryStoreCreatedEvent struct {
	OwnerAddr    string `json:"owner_addr"`
	StoreAddr    string `json:"store_addr"`
	MetadataAddr string `json:"metadata_addr"`
}

type FAEvent struct {
	StoreAddr string `json:"store_addr"`
}
