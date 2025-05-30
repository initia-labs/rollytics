package evm_nft

import abci "github.com/cometbft/cometbft/abci/types"

type CacheData struct {
	CollectionMap map[string]string
	NftMap        map[string]string
}

type EventWithHash struct {
	TxHash string
	abci.Event
}

type QueryCallResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

type QueryTokenUriData struct {
	CollectionAddr string
	TokenId        string
}
