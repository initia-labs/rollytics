package tx

import (
	"encoding/json"
	"errors"
	"regexp"

	abci "github.com/cometbft/cometbft/abci/types"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

const (
	InitBech32Regex = "^init1(?:[a-z0-9]{38}|[a-z0-9]{58})$"
	InitHexRegex    = "0x(?:[a-f1-9][a-f0-9]){1,64}"
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

func grepAddressesFromTx(events []abci.Event) (grepped []string, err error) {
	for _, event := range events {
		for _, attr := range event.Attributes {
			var addrs []string

			switch {
			case event.Type == evmtypes.EventTypeEVM && attr.Key == evmtypes.AttributeKeyLog:
				contractAddrs, err := extractAddressesFromEVMLog(attr.Value)
				if err != nil {
					return grepped, err
				}
				addrs = append(addrs, contractAddrs...)
			case isEvmModuleEvent(event.Type) && attr.Key == evmtypes.AttributeKeyContract:
				addrs = append(addrs, attr.Value)
			default:
				addrs = findAllBech32Address(attr.Value)
				addrs = append(addrs, findAllHexAddress(attr.Value)...)
			}

			for _, addr := range addrs {
				accAddr, err := util.AccAddressFromString(addr)
				if err != nil {
					return grepped, err
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

	addr, err := util.AccAddressFromString(log.Address)
	if err != nil {
		return addrs, err
	}
	addrs = append(addrs, addr.String())

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
		addr, err := util.AccAddressFromString(log.Topics[i])
		if err != nil {
			return addrs, err
		}
		addrs = append(addrs, addr.String())
	}

	return
}
