package utils

type BalanceChangeKey struct {
	Denom string
	Addr  string
}

// AddressWithID represents an address with its account ID
type AddressWithID struct {
	BechAddress string
	HexAddress  string
	Id          int64
}

const (
	EMPTY_ADDRESS      = "0x0000000000000000000000000000000000000000000000000000000000000000"
	EVM_TRANSFER_TOPIC = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

// EvmEventLog represents the EVM log structure found in "evm" type events
type EvmEventLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}
