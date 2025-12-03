package config

import (
	"fmt"
	"net/url"

	"github.com/initia-labs/rollytics/types"
)

type ChainConfig struct {
	ChainId              string
	VmType               types.VMType
	RpcUrls              []string
	RestUrls             []string
	JsonRpcUrls          []string
	AccountAddressPrefix string
	Environment          string
}

func (cc ChainConfig) Validate() error {
	// Chain ID validation
	if len(cc.ChainId) == 0 {
		return types.NewValidationError("CHAIN_ID", "required field is missing")
	}

	// RPC URL validation
	if len(cc.RpcUrls) == 0 {
		return types.NewValidationError("RPC_URL", "required field is missing")
	}
	for idx, rpcUrl := range cc.RpcUrls {
		if len(rpcUrl) == 0 {
			return types.NewValidationError("RPC_URL", fmt.Sprintf("URL at index %d is empty", idx))
		}
		if u, err := url.Parse(rpcUrl); err != nil {
			return types.NewValidationError("RPC_URL", fmt.Sprintf("invalid URL format at index %d: %s", idx, rpcUrl))
		} else if u.Scheme != "http" && u.Scheme != "https" {
			return types.NewValidationError("RPC_URL", fmt.Sprintf("URL at index %d must use http or https scheme, got: %s", idx, u.Scheme))
		}
	}

	// REST URL validation
	if len(cc.RestUrls) == 0 {
		return types.NewValidationError("REST_URL", "required field is missing")
	}
	for idx, restUrl := range cc.RestUrls {
		if len(restUrl) == 0 {
			return types.NewValidationError("REST_URL", fmt.Sprintf("URL at index %d is empty", idx))
		}
		if u, err := url.Parse(restUrl); err != nil {
			return types.NewInvalidValueError("REST_URL", restUrl, fmt.Sprintf("invalid URL at index %d: %v", idx, err))
		} else if u.Scheme != "http" && u.Scheme != "https" {
			return types.NewInvalidValueError("REST_URL", restUrl, fmt.Sprintf("URL at index %d must use http or https scheme, got: %s", idx, u.Scheme))
		}
	}

	// EVM specific validation
	if cc.VmType == types.EVM {
		if len(cc.JsonRpcUrls) == 0 {
			return types.NewValidationError("JSON_RPC_URL", "is required for EVM")
		}
		for i, jsonRpcUrl := range cc.JsonRpcUrls {
			if len(jsonRpcUrl) == 0 {
				return types.NewValidationError("JSON_RPC_URL", fmt.Sprintf("URL at index %d is empty", i))
			}
			if u, err := url.Parse(jsonRpcUrl); err != nil {
				return types.NewInvalidValueError("JSON_RPC_URL", jsonRpcUrl, fmt.Sprintf("invalid URL at index %d: %v", i, err))
			} else if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
				return types.NewInvalidValueError("JSON_RPC_URL", jsonRpcUrl, fmt.Sprintf("URL at index %d must use http, https, ws or wss scheme, got: %s", i, u.Scheme))
			}
		}
	}

	// VM Type validation
	switch cc.VmType {
	case types.MoveVM, types.WasmVM, types.EVM:
		// Valid VM types
	default:
		return types.NewInvalidValueError("VM_TYPE", fmt.Sprintf("%v", cc.VmType), "must be 'move', 'wasm', or 'evm'")
	}

	// Account address prefix validation
	if len(cc.AccountAddressPrefix) == 0 {
		return types.NewValidationError("ACCOUNT_ADDRESS_PREFIX", "is required")
	}

	return nil
}

// GetRpcUrl returns the first RPC URL (for backward compatibility)
func (cc ChainConfig) GetRpcUrl() string {
	if len(cc.RpcUrls) == 0 {
		return ""
	}
	return cc.RpcUrls[0]
}

// GetRestUrl returns the first REST URL (for backward compatibility)
func (cc ChainConfig) GetRestUrl() string {
	if len(cc.RestUrls) == 0 {
		return ""
	}
	return cc.RestUrls[0]
}

// GetJsonRpcUrl returns the first JSON-RPC URL (for backward compatibility)
func (cc ChainConfig) GetJsonRpcUrl() string {
	if len(cc.JsonRpcUrls) == 0 {
		return ""
	}
	return cc.JsonRpcUrls[0]
}
