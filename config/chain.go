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
		return fmt.Errorf("REST_URL is required")
	}
	if u, err := url.Parse(cc.RestUrl); err != nil {
		return fmt.Errorf("REST_URL(%s) is invalid: %w", cc.RestUrl, err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("REST_URL must use http or https scheme, got: %s", u.Scheme)
	}

	// EVM specific validation
	if cc.VmType == types.EVM {
		if len(cc.JsonRpcUrl) == 0 {
			return fmt.Errorf("JSON_RPC_URL is required for EVM")
		}
		if u, err := url.Parse(cc.JsonRpcUrl); err != nil {
			return fmt.Errorf("JSON_RPC_URL(%s) is invalid: %w", cc.JsonRpcUrl, err)
		} else if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
			return fmt.Errorf("JSON_RPC_URL must use http, https, ws or wss scheme, got: %s", u.Scheme)
		}
	}

	// VM Type validation
	switch cc.VmType {
	case types.MoveVM, types.WasmVM, types.EVM:
		// Valid VM types
	default:
		return fmt.Errorf("invalid VM_TYPE: must be 'move', 'wasm', or 'evm'")
	}

	// Account address prefix validation
	if len(cc.AccountAddressPrefix) == 0 {
		return fmt.Errorf("ACCOUNT_ADDRESS_PREFIX is required")
	}

	return nil
}
