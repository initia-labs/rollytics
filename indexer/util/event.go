package util

import (
	"encoding/base64"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/initia-labs/rollytics/indexer/types"
)

func ExtractEvents(block types.ScrapedBlock, eventType string) (events []types.ParsedEvent, err error) {
	events = parseEvents(block.PreBlock, "", eventType)
	events = append(events, parseEvents(block.BeginBlock, "", eventType)...)

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
