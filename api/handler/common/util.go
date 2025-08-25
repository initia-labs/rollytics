package common

import (
	"github.com/initia-labs/rollytics/util"
)

// normalizeCollectionAddr parses collection address (supports both hex and bech32)
func normalizeCollectionAddr(collectionAddr string) ([]byte, error) {
	accAddr, err := util.AccAddressFromString(collectionAddr)
	if err != nil {
		return nil, err
	}
	return accAddr.Bytes(), nil
}
