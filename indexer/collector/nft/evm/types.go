package evm

import abci "github.com/cometbft/cometbft/abci/types"

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
