package types

type CacheData struct {
	CollectionMap map[string]string
	NftMap        map[string]string
}

type ErrorResponse struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}
