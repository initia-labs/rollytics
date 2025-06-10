package common

import (
	"encoding/hex"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func EnsureLowerHexAddress(addr string) (string, error) {
	addr = strings.ToLower(addr)
	if _, err := hex.DecodeString(addr); err != nil {
		return "", fmt.Errorf("invalid hex address: %s", addr)
	}
	return addr, nil
}

func AccAddressFromString(addrStr string) (sdk.AccAddress, error) {
	if !strings.HasPrefix(addrStr, "0x") {
		return sdk.AccAddressFromBech32(addrStr)
	}

	hexStr := strings.ToLower(strings.TrimLeft(strings.TrimPrefix(addrStr, "0x"), "0"))

	if len(hexStr) <= 40 {
		hexStr = strings.Repeat("0", 40-len(hexStr)) + hexStr
	} else if len(hexStr) <= 64 {
		hexStr = strings.Repeat("0", 64-len(hexStr)) + hexStr
	} else {
		return nil, fmt.Errorf("invalid address string: %s", addrStr)
	}

	return hex.DecodeString(hexStr)
}
