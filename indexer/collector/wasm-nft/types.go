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

type QueryWasmContractResponse struct {
	ContractInfo struct {
		Creator string `json:"creator"`
	} `json:"contract_info"`
}
