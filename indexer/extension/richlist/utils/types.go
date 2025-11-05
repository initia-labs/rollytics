package utils

import (
	"encoding/json"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

const (
	COSMOS_TRANSFER_EVENT = "transfer"
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
