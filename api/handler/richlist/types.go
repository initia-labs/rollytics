package richlist

import (
	"github.com/initia-labs/rollytics/api/handler/common"
)

type TokenHoldersResponse struct {
	Holders    []TokenHolder             `json:"holders"`
	Pagination common.PaginationResponse `json:"pagination"`
}

type TokenHolder struct {
	Account string `json:"account"`
	Amount  string `json:"amount"`
}
