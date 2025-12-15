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

// EvmEventLog represents the EVM log structure found in "evm" type events
type EvmEventLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}
