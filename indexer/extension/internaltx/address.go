package internaltx

import (
	"strings"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

const (
	addressPrefix = "0x000000000000000000000000"
)

func GrepAddressesFromEvmInternalTx(evmInternalTx types.EvmInternalTx) (grepped []string, err error) {
	var addrs = make(map[string]interface{})

	convertToAccAddr := func(addrs map[string]interface{}, grepped []string) ([]string, error) {
		for addr := range addrs {
			accAddr, err := util.AccAddressFromString(addr)
			if err != nil {
				return grepped, err
			}
			grepped = append(grepped, accAddr.String())
		}
		return grepped, nil
	}

	if evmInternalTx.From != "" {
		addrs[evmInternalTx.From] = nil
	}
	if evmInternalTx.To != "" {
		addrs[evmInternalTx.To] = nil
	}
	input := evmInternalTx.Input
	if len(input) < 10 {
		return convertToAccAddr(addrs, grepped)
	}

	input = input[10:] // Remove function selector
	for i := 0; i+32 <= len(input); i += 32 {
		argStr := string(input[i : i+32])
		if strings.HasPrefix(argStr, addressPrefix) {
			addrs[argStr] = nil
		}
	}

	return convertToAccAddr(addrs, grepped)
}
