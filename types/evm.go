package types

// EVM constants
const (
	// EvmEmptyAddress is the zero address used in EVM transfers (mint/burn)
	EvmEmptyAddress = "0x0000000000000000000000000000000000000000000000000000000000000000"
	// EvmTransferTopic is the keccak256 hash of Transfer(address,address,uint256) event signature
	EvmTransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

type QueryEvmTxsResponse struct {
	Result []EvmTx `json:"result"`
}

// EvmContractByDenomResponse represents the response from /minievm/evm/v1/contracts/by_denom
type EvmContractByDenomResponse struct {
	Address string `json:"address"`
}
