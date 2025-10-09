package config

import (
	"fmt"
	"net/url"

	"github.com/initia-labs/rollytics/types"
)

type ChainConfig struct {
	ChainId              string
	VmType               types.VMType
	RpcUrl               string
	RestUrl              string
	JsonRpcUrl           string
	AccountAddressPrefix string
	Environment          string
}

func (cc ChainConfig) Validate() error {
	// Chain ID validation
	if len(cc.ChainId) == 0 {
		return types.NewValidationError("CHAIN_ID", "required field is missing")
	}

	// RPC URL validation
	if len(cc.RpcUrl) == 0 {
		return types.NewValidationError("RPC_URL", "required field is missing")
	}
	if u, err := url.Parse(cc.RpcUrl); err != nil {
		return types.NewValidationError("RPC_URL", fmt.Sprintf("invalid URL format: %s", cc.RpcUrl))
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return types.NewValidationError("RPC_URL", fmt.Sprintf("must use http or https scheme, got: %s", u.Scheme))
	}

	// REST URL validation
	if len(cc.RestUrl) == 0 {
		return types.NewValidationError("REST_URL", "required field is missing")
	}
	if u, err := url.Parse(cc.RestUrl); err != nil {
		return types.NewInvalidValueError("REST_URL", cc.RestUrl, fmt.Sprintf("invalid URL: %v", err))
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return types.NewInvalidValueError("REST_URL", cc.RestUrl, fmt.Sprintf("must use http or https scheme, got: %s", u.Scheme))
	}

	// EVM specific validation
	if cc.VmType == types.EVM {
		if len(cc.JsonRpcUrl) == 0 {
			return types.NewValidationError("JSON_RPC_URL", "is required for EVM")
		}
		if u, err := url.Parse(cc.JsonRpcUrl); err != nil {
			return types.NewInvalidValueError("JSON_RPC_URL", cc.JsonRpcUrl, fmt.Sprintf("invalid URL: %v", err))
		} else if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
			return types.NewInvalidValueError("JSON_RPC_URL", cc.JsonRpcUrl, fmt.Sprintf("must use http, https, ws or wss scheme, got: %s", u.Scheme))
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
