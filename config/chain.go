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
	if len(cc.ChainId) == 0 {
		return fmt.Errorf("CHAIN_ID is required")
	}
	if len(cc.RpcUrl) == 0 {
		return fmt.Errorf("RPC_URL is required")
	}
	if _, err := url.Parse(cc.RpcUrl); err != nil {
		return fmt.Errorf("RPC_URL(%s) is invalid: %s", cc.RpcUrl, err)
	}
	if len(cc.RestUrl) == 0 {
		return fmt.Errorf("REST_URL is required")
	}
	if _, err := url.Parse(cc.RestUrl); err != nil {
		return fmt.Errorf("REST_URL(%s) is invalid: %s", cc.RestUrl, err)
	}
	if cc.VmType == types.EVM {
		if len(cc.JsonRpcUrl) == 0 {
			return fmt.Errorf("JSON_RPC_URL is required")
		}
		if _, err := url.Parse(cc.JsonRpcUrl); err != nil {
			return fmt.Errorf("JSON_RPC_URL(%s) is invalid: %s", cc.JsonRpcUrl, err)
		}
	}
	return nil
}
