package util

import (
	"encoding/hex"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	commontypes "github.com/initia-labs/rollytics/types"
)

func AccAddressFromString(addrStr string) (sdk.AccAddress, error) {
	if !strings.HasPrefix(addrStr, "0x") {
		addr, err := sdk.AccAddressFromBech32(addrStr)
		if err == nil {
			return addr, nil
		}
	}

	hexStr := strings.ToLower(strings.TrimLeft(strings.TrimPrefix(addrStr, "0x"), "0"))

	if len(hexStr) <= 40 { //nolint:gocritic
		hexStr = strings.Repeat("0", 40-len(hexStr)) + hexStr
	} else if len(hexStr) <= 64 {
		hexStr = strings.Repeat("0", 64-len(hexStr)) + hexStr
	} else {
		return nil, commontypes.NewInvalidValueError("address", addrStr, "invalid address format")
	}

	return hex.DecodeString(hexStr)
}

func HexToBytes(hexStr string) ([]byte, error) {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	if hexStr == "" {
		return []byte{}, nil
	}
	// Pad with leading zero if hex string has odd length
	if len(hexStr)%2 == 1 {
		hexStr = "0" + hexStr
	}
	return hex.DecodeString(hexStr)
}

func BytesToHex(b []byte) string {
	return hex.EncodeToString(b)
}

func BytesToHexWithPrefix(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}
