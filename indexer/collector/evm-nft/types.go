package evm_nft

import "time"

type CacheData struct {
	ColNames  map[string]string
	TokenUris map[string]map[string]string
}

type QueryCallResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

type CollectionCreationInfo struct {
	Height    int64
	Timestamp time.Time
	Creator   string
}
