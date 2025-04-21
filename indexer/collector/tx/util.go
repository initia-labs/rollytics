package tx

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

const (
	InitBech32Regex = "^init1(?:[a-z0-9]{38}|[a-z0-9]{58})$"
	InitHexRegex    = "0x(?:[a-f1-9][a-f0-9]*){1,64}"
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
