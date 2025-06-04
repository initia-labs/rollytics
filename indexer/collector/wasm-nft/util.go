package wasm_nft

import (
	"encoding/base64"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
)

func extractEvents(block indexertypes.ScrappedBlock, eventType string) (events []indexertypes.ParsedEvent, err error) {
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

func parseEvents(evts []abci.Event, txHash string, eventType string) (parsedEvts []indexertypes.ParsedEvent) {
	for _, evt := range evts {
		if evt.Type != eventType {
			continue
		}

		parsedEvts = append(parsedEvts, parseEvent(evt, txHash))
	}

	return
}

func parseEvent(evt abci.Event, txHash string) indexertypes.ParsedEvent {
	attributes := make(map[string]string)
	for _, attr := range evt.Attributes {
		attributes[attr.Key] = attr.Value
	}
	return indexertypes.ParsedEvent{
		TxHash:     txHash,
		Type:       evt.Type,
		Attributes: attributes,
	}
}
