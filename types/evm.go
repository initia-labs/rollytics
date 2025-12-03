package types

type QueryEvmTxsResponse struct {
	Result []EvmTx `json:"result"`
}

// EvmContractByDenomResponse represents the response from /minievm/evm/v1/contracts/by_denom
type EvmContractByDenomResponse struct {
	Address string `json:"address"`
}
