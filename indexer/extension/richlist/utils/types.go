package utils

import "encoding/json"

const (
	COSMOS_TRANSFER_EVENT = "transfer"
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

// CosmosAccount represents a simplified account from the Cosmos SDK.
// It supports multiple account formats:
// - BaseAccount: { "address": "init1...", ... }
// - ModuleAccount, ContractAccount, and others: { "base_account": { "address": "init1...", ... }, ... }
type CosmosAccount struct {
	Type        string   `json:"@type"`
	Address     string   `json:"address"`
	Permissions []string `json:"permissions,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to support both account types
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

	return nil
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
