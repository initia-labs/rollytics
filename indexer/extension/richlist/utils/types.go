package utils

import (
	"encoding/json"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
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

func getAddressFromAccount(account *codectypes.Any) (string, error) {
	// Handle accounts with base_account field (e.g., TableAccount, ContractAccount, etc.)
	// Try to unmarshal as JSON to extract address from base_account
	var raw map[string]any
	if err := json.Unmarshal(account.Value, &raw); err != nil {
		return "", err
	}

	// Check if this account has a base_account field
	if baseAccountMap, ok := raw["base_account"].(map[string]any); ok {
		if address, ok := baseAccountMap["address"].(string); ok {
			return address, nil
		}
	}

	// Fallback: try direct address field (for BaseAccount)
	if address, ok := raw["address"].(string); ok {
		return address, nil
	}

	return "", fmt.Errorf("invalid account type")
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

// QueryAccountsResponse represents the accounts query response with custom pagination
type QueryAccountsResponse struct {
	Accounts   []*codectypes.Any `json:"accounts"`
	Pagination *Pagination       `json:"pagination,omitempty"`
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
