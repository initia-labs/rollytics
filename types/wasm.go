package types

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
