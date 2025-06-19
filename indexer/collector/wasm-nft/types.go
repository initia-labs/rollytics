package wasm_nft

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

type QueryMinterResponse struct {
	Data struct {
		Minter string `json:"minter"`
	} `json:"data"`
}
