package common

import (
	"errors"
	"strings"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// normalizeCollectionAddr parses collection address based on VM type
func normalizeCollectionAddr(collectionAddr string, config *config.ChainConfig) ([]byte, error) {
	switch config.VmType {
	case types.MoveVM:
		accAddr, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			return nil, err
		}
		return accAddr.Bytes(), nil
	case types.EVM:
		if !strings.HasPrefix(collectionAddr, "0x") {
			return nil, errors.New("collection address should be hex address")
		}
		return util.HexToBytes(strings.ToLower(collectionAddr))
	case types.WasmVM:
		if !strings.HasPrefix(collectionAddr, config.AccountAddressPrefix) {
			return nil, errors.New("collection address should be bech32 address")
		}
		accAddr, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			return nil, err
		}
		return accAddr.Bytes(), nil
	default:
		return nil, errors.New("unsupported vm type")
	}
}
