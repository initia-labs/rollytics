package evm_nft

type CacheData struct {
	ColNames  map[string]string
	TokenUris map[string]map[string]string
}

type QueryCallResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}
