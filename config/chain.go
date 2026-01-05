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
	if err := cc.validateChainId(); err != nil {
		return err
	}
	if err := cc.validateRpcUrls(); err != nil {
		return err
	}
	if err := cc.validateRestUrls(); err != nil {
		return err
	}
	if err := cc.validateEvmUrls(); err != nil {
		return err
	}
	if err := cc.validateVmType(); err != nil {
		return err
	}
	if err := cc.validateAccountPrefix(); err != nil {
		return err
	}
	return nil
}

func (cc ChainConfig) validateChainId() error {
	if len(cc.ChainId) == 0 {
		return types.NewValidationError("CHAIN_ID", "required field is missing")
	}
	return nil
}

func (cc ChainConfig) validateRpcUrls() error {
	if len(cc.RpcUrls) == 0 {
		return types.NewValidationError("RPC_URL", "required field is missing")
	}
	for idx, rpcUrl := range cc.RpcUrls {
		if err := validateHttpUrl("RPC_URL", rpcUrl, idx); err != nil {
			return err
		}
	}
	return nil
}

func (cc ChainConfig) validateRestUrls() error {
	if len(cc.RestUrls) == 0 {
		return types.NewValidationError("REST_URL", "required field is missing")
	}
	for idx, restUrl := range cc.RestUrls {
		if err := validateHttpUrl("REST_URL", restUrl, idx); err != nil {
			return err
		}
	}
	return nil
}

func (cc ChainConfig) validateEvmUrls() error {
	if cc.VmType != types.EVM {
		return nil
	}
	if len(cc.JsonRpcUrls) == 0 {
		return types.NewValidationError("JSON_RPC_URL", "is required for EVM")
	}
	for i, jsonRpcUrl := range cc.JsonRpcUrls {
		if len(jsonRpcUrl) == 0 {
			return types.NewValidationError("JSON_RPC_URL", fmt.Sprintf("URL at index %d is empty", i))
		}
		u, err := url.Parse(jsonRpcUrl)
		if err != nil {
			return types.NewInvalidValueError("JSON_RPC_URL", jsonRpcUrl, fmt.Sprintf("invalid URL at index %d: %v", i, err))
		}
		if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "ws" && u.Scheme != "wss" {
			return types.NewInvalidValueError("JSON_RPC_URL", jsonRpcUrl, fmt.Sprintf("URL at index %d must use http, https, ws or wss scheme, got: %s", i, u.Scheme))
		}
	}
	return nil
}

func (cc ChainConfig) validateVmType() error {
	switch cc.VmType {
	case types.MoveVM, types.WasmVM, types.EVM:
		return nil
	default:
		return types.NewInvalidValueError("VM_TYPE", fmt.Sprintf("%v", cc.VmType), "must be 'move', 'wasm', or 'evm'")
	}
}

func (cc ChainConfig) validateAccountPrefix() error {
	if len(cc.AccountAddressPrefix) == 0 {
		return types.NewValidationError("ACCOUNT_ADDRESS_PREFIX", "is required")
	}
	return nil
}

func validateHttpUrl(fieldName, urlStr string, idx int) error {
	if len(urlStr) == 0 {
		return types.NewValidationError(fieldName, fmt.Sprintf("URL at index %d is empty", idx))
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		if fieldName == "RPC_URL" {
			return types.NewValidationError(fieldName, fmt.Sprintf("invalid URL format at index %d: %s", idx, urlStr))
		}
		return types.NewInvalidValueError(fieldName, urlStr, fmt.Sprintf("invalid URL at index %d: %v", idx, err))
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		if fieldName == "RPC_URL" {
			return types.NewValidationError(fieldName, fmt.Sprintf("URL at index %d must use http or https scheme, got: %s", idx, u.Scheme))
		}
		return types.NewInvalidValueError(fieldName, urlStr, fmt.Sprintf("URL at index %d must use http or https scheme, got: %s", idx, u.Scheme))
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
