package tx

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	movetypes "github.com/initia-labs/initia/x/move/types"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

const (
	InitBech32Regex = "^init1(?:[a-z0-9]{38}|[a-z0-9]{58})$"
	InitHexRegex    = "^0x(?:[a-fA-F0-9]{1,64})$"
	MoveHexRegex    = "0x(?:[a-fA-F0-9]{1,64})"
)

var (
	regexInitBech = regexp.MustCompile(InitBech32Regex)
	regexHex      = regexp.MustCompile(InitHexRegex)
	regexMoveHex  = regexp.MustCompile(MoveHexRegex)
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

func findAllMoveHexAddress(attr string) []string {
	return regexMoveHex.FindAllString(attr, -1)
}

func grepAddressesFromTx(events []abci.Event) (grepped []string, err error) {
	for _, event := range events {
		for _, attr := range event.Attributes {
			var addrs []string

			switch {
			case event.Type == movetypes.EventTypeMove && attr.Key == movetypes.AttributeKeyData:
				addrs = append(addrs, findAllMoveHexAddress(attr.Value)...)
			case event.Type == evmtypes.EventTypeEVM && attr.Key == evmtypes.AttributeKeyLog:
				var log evmtypes.Log
				if err = json.Unmarshal([]byte(attr.Value), &log); err != nil {
					return grepped, err
				}

				addrs = append(addrs, log.Address)
				for idx, topic := range log.Topics {
					if idx > 0 && strings.HasPrefix(topic, "0x000000000000000000000000") {
						addrs = append(addrs, topic)
					}
				}

			default:
				for _, attrVal := range strings.Split(attr.Value, ",") {
					addrs = append(addrs, findAllBech32Address(attrVal)...)
					addrs = append(addrs, findAllHexAddress(attrVal)...)
				}
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

func grepAddressesFromEvmTx(evmTx EvmTx) (grepped []string, err error) {
	var addrs []string

	if evmTx.From != "" {
		grepped = append(grepped, evmTx.From)
	}
	if evmTx.To != "" {
		grepped = append(grepped, evmTx.To)
	}

	for _, log := range evmTx.Logs {
		addrs = append(addrs, log.Address)
		for idx, topic := range log.Topics {
			if idx > 0 && strings.HasPrefix(topic, "0x000000000000000000000000") {
				addrs = append(addrs, topic)
			}
		}
	}

	for _, addr := range addrs {
		accAddr, err := util.AccAddressFromString(addr)
		if err != nil {
			return grepped, err
		}
		grepped = append(grepped, accAddr.String())
	}

	return
}
