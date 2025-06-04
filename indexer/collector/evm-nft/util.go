package evm_nft

import (
	"errors"
	"math/big"
	"strings"

	evmtypes "github.com/initia-labs/minievm/x/evm/types"
)

func isEvmNftLog(log evmtypes.Log) bool {
	return len(log.Topics) == 4 && log.Topics[0] == nftTopic && log.Data == "0x"
}

func convertHexStringToDecString(hex string) (string, error) {
	hex = strings.TrimPrefix(hex, "0x")
	bi, ok := new(big.Int).SetString(hex, 16)
	if !ok {
		return "", errors.New("failed to convert hex to dec")
	}
	return bi.String(), nil
}
