package tx

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

const (
	InitBech32Regex = "^init1(?:[a-z0-9]{38}|[a-z0-9]{58})$"
	InitHexRegex    = "0x(?:[a-f1-9][a-f0-9]*){1,64}"
	transferTopic   = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

var (
	regexInitBech = regexp.MustCompile(InitBech32Regex)
	regexHex      = regexp.MustCompile(InitHexRegex)
)

func getSeqInfo(chainId string, name string, tx *gorm.DB) (seqInfo types.CollectedSeqInfo, err error) {
	if res := tx.Where("chain_id = ? AND name = ?", chainId, name).Take(&seqInfo); res.Error != nil {
		// initialize if not found
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			seqInfo = types.CollectedSeqInfo{
				ChainId:  chainId,
				Name:     name,
				Sequence: 0,
			}
		} else {
			return seqInfo, res.Error
		}
	}

	return seqInfo, nil
}

func findAllBech32Address(attr string) []string {
	return regexInitBech.FindAllString(attr, -1)
}

func findAllHexAddress(attr string) []string {
	return regexHex.FindAllString(attr, -1)
}

func accAddressFromString(addrStr string) (addr sdk.AccAddress, err error) {
	if strings.HasPrefix(addrStr, "0x") {
		addrStr = strings.TrimPrefix(addrStr, "0x")

		// add padding
		if len(addrStr) <= 40 {
			addrStr = strings.Repeat("0", 40-len(addrStr)) + addrStr
		} else if len(addrStr) <= 64 {
			addrStr = strings.Repeat("0", 64-len(addrStr)) + addrStr
		} else {
			return nil, fmt.Errorf("invalid address string: %s", addrStr)
		}

		if addr, err = hex.DecodeString(addrStr); err != nil {
			return
		}
	} else if addr, err = sdk.AccAddressFromBech32(addrStr); err != nil {
		return
	}

	return
}

func convertContractAddressToBech32(addr string) (string, error) {
	accAddr, err := sdk.AccAddressFromHexUnsafe(strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(addr, "0x"), "000000000000000000000000")))
	if err != nil {
		return "", err
	}
	return accAddr.String(), nil
}

func parseEvent(event abci.Event) indexertypes.ParsedEvent {
	attrMap := make(map[string]string)
	for _, attr := range event.Attributes {
		attrMap[attr.Key] = attr.Value
	}

	return indexertypes.ParsedEvent{
		Type:       event.Type,
		Attributes: attrMap,
	}
}

func grepAddressesFromTx(events []abci.Event) (grepped []string) {
	for _, event := range events {
		for _, attr := range event.Attributes {
			var addrs []string

			switch {
			case event.Type == evmtypes.EventTypeEVM && attr.Key == evmtypes.AttributeKeyLog:
				contractAddrs, err := extractAddressesFromEVMLog(attr.Value)
				if err != nil {
					continue
				}
				addrs = append(addrs, contractAddrs...)
			case isEvmModuleEvent(event.Type) && attr.Key == evmtypes.AttributeKeyContract:
				addr, err := convertContractAddressToBech32(attr.Value)
				if err != nil {
					continue
				}
				addrs = append(addrs, addr)
			default:
				addrs = findAllBech32Address(attr.Value)
				addrs = append(addrs, findAllHexAddress(attr.Value)...)
			}

			for _, addr := range addrs {
				accAddr, err := accAddressFromString(addr)
				if err != nil {
					continue
				}
				grepped = append(grepped, accAddr.String())
			}
		}
	}

	return
}

// isEvmModuleEvent checks if the event type is from evm module except evmtypes.EventTypeEVM.
// return true if it is, false otherwise.
func isEvmModuleEvent(eventType string) bool {
	switch eventType {
	case evmtypes.EventTypeCall, evmtypes.EventTypeCreate,
		evmtypes.EventTypeContractCreated, evmtypes.EventTypeERC20Created,
		evmtypes.EventTypeERC721Created, evmtypes.EventTypeERC721Minted, evmtypes.EventTypeERC721Burned:
		return true
	default:
		return false
	}
}

func extractAddressesFromEVMLog(attrVal string) (addrs []string, err error) {
	log := evmtypes.Log{}
	if err = json.Unmarshal([]byte(attrVal), &log); err != nil {
		return
	}
	var addr string
	addr, err = convertContractAddressToBech32(log.Address)
	if err == nil {
		addrs = append(addrs, addr)
	}

	// if the topic is about transfer, we need to extract the addresses from the topics.
	if log.Topics == nil { // no topic
		return
	}
	topicLen := len(log.Topics)
	if topicLen < 2 { // no data to extract
		return
	}
	if log.Topics[0] != transferTopic { // topic is not about transfer
		return
	}

	for i := 1; i < topicLen; i++ {
		if i == 3 { // if index is 3, it means index indicates the amount, not address. need break
			break
		}
		addr, err = convertContractAddressToBech32(log.Topics[i])
		if err != nil {
			continue
		}
		addrs = append(addrs, addr)
	}

	return
}

func uniqueAppend(slice []string, elem string) []string {
	for _, e := range slice {
		if e == elem {
			return slice
		}
	}
	return append(slice, elem)
}
