package evm

type QueryCallResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

type QueryTokenUriData struct {
	CollectionAddr string
	TokenId        string
}
