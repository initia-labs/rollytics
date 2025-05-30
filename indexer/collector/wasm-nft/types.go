package wasm_nft

type CacheData struct {
	CollectionMap map[string]CacheCollectionInfo
}

type CacheCollectionInfo struct {
	Name    string
	Creator string
}

type QueryContractInfoResponse struct {
	Data struct {
		Name string `json:"name"`
	} `json:"data"`
}

type QueryCollectionInfoResponse struct {
	Data struct {
		Creator string `json:"creator"`
	} `json:"data"`
}
