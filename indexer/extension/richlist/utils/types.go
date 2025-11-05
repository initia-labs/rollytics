package utils

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

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

// CosmosCoin represents a coin with denom and amount
type CosmosCoin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// Pagination represents pagination info with Total as string (as returned by JSON API)
type Pagination struct {
	NextKey []byte `json:"next_key"`
	Total   string `json:"total"`
}

// CosmosAccountsResponse represents the response from /cosmos/auth/v1beta1/accounts
type CosmosAccountsResponse struct {
	Accounts   []CosmosAccount `json:"accounts"`
	Pagination Pagination      `json:"pagination,omitempty"`
}

// QueryAllBalancesResponse represents the balances query response with custom pagination
type QueryAllBalancesResponse struct {
	Balances   []sdk.Coin  `json:"balances"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

// QueryModuleAccountsResponse represents the module accounts query response
type QueryModuleAccountsResponse struct {
	Accounts []ModuleAccount `json:"accounts"`
}

// ModuleAccount represents a module account with address and permissions
type ModuleAccount struct {
	Address     string   `json:"address"`
	Permissions []string `json:"permissions"`
}
