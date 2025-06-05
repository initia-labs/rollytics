package util

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/initia-labs/rollytics/indexer/types"
)

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

func ExtractEvents(block types.ScrapedBlock, eventType string) (events []types.ParsedEvent, err error) {
	events = parseEvents(block.BeginBlock, "", eventType)

	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return events, err
		}
		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))
		txRes := block.TxResults[txIndex]
		events = append(events, parseEvents(txRes.Events, txHash, eventType)...)
	}

	events = append(events, parseEvents(block.EndBlock, "", eventType)...)

	return events, nil
}

func parseEvents(events []abci.Event, txHash string, eventType string) (parsed []types.ParsedEvent) {
	for _, event := range events {
		if event.Type != eventType {
			continue
		}

		parsed = append(parsed, parseEvent(event, txHash))
	}

	return
}

func parseEvent(event abci.Event, txHash string) types.ParsedEvent {
	attrMap := make(map[string]string)
	for _, attr := range event.Attributes {
		attrMap[attr.Key] = attr.Value
	}

	return types.ParsedEvent{
		TxHash:  txHash,
		Event:   event,
		AttrMap: attrMap,
	}
}
