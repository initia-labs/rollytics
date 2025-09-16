package wasm_nft

import indexerutil "github.com/initia-labs/rollytics/indexer/util"

const (
	EventTypeWasm             = "wasm"
	CustomContractEventPrefix = "wasm-"

	EventAttrNftMint     = "mint"
	EventAttrNftBurn     = "burn"
	EventAttrNftTransfer = "transfer_nft"
	EventAttrNftSend     = "send_nft"
)

type CacheData struct {
	ColInfos map[string]CollectionInfo
}

type CollectionInfo struct {
	Name    string
	Creator string
}

type QueryContractInfoResponse struct {
	Data struct {
		Name string `json:"name"`
	} `json:"data"`
}

// WasmEventMatcher matches WASM events for backward compatibility
// Reference: https://github.com/CosmWasm/wasmd/blob/66f968e8f17957caeebb20ddb41e687a77944002/x/wasm/keeper/events.go#L42C1
// It matches events that either:
// 1. Have the "wasm-" prefix (new format)
// 2. Are exactly "wasm" (legacy format)
func WasmEventMatcher(eventType, targetType string) bool {
	// Check if the eventType matches the targetType pattern
	return indexerutil.PrefixMatch(eventType, targetType) || indexerutil.ExactMatch(eventType, EventTypeWasm)
}
