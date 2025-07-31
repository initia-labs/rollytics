package internaltx

import (
	"strings"

	"github.com/initia-labs/rollytics/util"
)

const (
	addressPrefix = "0x000000000000000000000000"
)

func GrepAddressesFromEvmInternalTx(evmInternalTx EvmInternalTx) (grepped []string, err error) {
	addrs := make(map[string]interface{})

	if evmInternalTx.From != "" {
		addrs[evmInternalTx.From] = nil
	}
	if evmInternalTx.To != "" {
		addrs[evmInternalTx.To] = nil
	}
	input := evmInternalTx.Input

	if len(input) > 10 {
		input = input[10:] // Remove function selector
		for i := 0; i+32 <= len(input); i += 32 {
			argStr := string(input[i : i+32])
			if strings.HasPrefix(argStr, addressPrefix) {
				addrs[argStr] = nil
			}
		}
	}

	for addr := range addrs {
		accAddr, err := util.AccAddressFromString(addr)
		if err != nil {
			return grepped, err
		}
		grepped = append(grepped, accAddr.String())
	}
	return grepped, nil
}
