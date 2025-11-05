package utils

import (
	"encoding/json"
	"fmt"

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

// CosmosAccount represents a simplified account from the Cosmos SDK.
// It supports multiple account formats:
// - BaseAccount: { "address": "init1...", ... }
// - ModuleAccount, ContractAccount, and others: { "base_account": { "address": "init1...", ... }, ... }
type CosmosAccount struct {
	Type        string   `json:"@type"`
	Address     string   `json:"address"`
	Permissions []string `json:"permissions,omitempty"`
}

func (a *CosmosAccount) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a map to check the structure
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Check if this is a non BaseAccount
	if baseAccount, ok := raw["base_account"].(map[string]any); ok {
		if address, ok := baseAccount["address"].(string); ok {
			a.Address = address
			return nil
		}
	}

	// Otherwise, try direct address field (BaseAccount)
	if address, ok := raw["address"].(string); ok {
		a.Address = address
		return nil
	}

	return fmt.Errorf("no address field found in account JSON")
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
	Balances   []sdk.Coin `json:"balances"`
	Pagination Pagination `json:"pagination,omitempty"`
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
