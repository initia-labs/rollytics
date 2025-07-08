package common

import (
	"errors"
	"strings"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
)

func validateCollectionAddr(collectionAddr string, config *config.ChainConfig) error {
	switch config.VmType {
	case types.MoveVM, types.EVM:
		if !strings.HasPrefix(collectionAddr, "0x") {
			return errors.New("collection address should be hex address")
		}
	case types.WasmVM:
		if !strings.HasPrefix(collectionAddr, config.AccountAddressPrefix) {
			return errors.New("collection address should be bech32 address")
		}
	}

	return nil
}
