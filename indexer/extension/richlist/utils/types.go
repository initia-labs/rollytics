package utils

const (
	EMPTY_ADDRESS = "0x0000000000000000000000000000000000000000000000000000000000000000"

	COSMOS_EVM_EVENT      = "evm"
	COSMOS_TRANSFER_EVENT = "transfer"

	EVM_TRANSFER_TOPIC = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

type BalanceChangeKey struct {
	Asset string
	Addr  string
}

// AddressWithID represents an address with its account ID
type AddressWithID struct {
	Address   string
	AccountID int64
}

type CosmosEventEvmLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

// CosmosAccount represents a simplified account from the Cosmos SDK
type CosmosAccount struct {
	Address string `json:"address"`
}

type Pagination struct {
	NextKey []byte `json:"next_key"`
	Total   string `json:"total"`
}

// CosmosAccountsResponse represents the response from /cosmos/auth/v1beta1/accounts
type CosmosAccountsResponse struct {
	Accounts   []CosmosAccount `json:"accounts"`
	Pagination Pagination      `json:"pagination,omitempty"`
}

// CosmosCoin represents a coin with denom and amount
type CosmosCoin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// CosmosBalancesResponse represents the response from /cosmos/bank/v1beta1/balances/{address}
type CosmosBalancesResponse struct {
	Balances   []CosmosCoin `json:"balances"`
	Pagination Pagination   `json:"pagination,omitempty"`
}
