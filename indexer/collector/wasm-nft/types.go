package wasm_nft

type CacheData struct {
	ColInfos map[string]CollectionInfo
}

type CollectionInfo struct {
	Name    string
	Creator string
}
