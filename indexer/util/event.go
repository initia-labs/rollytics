package util

import (
	"encoding/base64"
	"fmt"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"

	"github.com/initia-labs/rollytics/indexer/types"
)

// EventMatcher defines how to match events
type EventMatcher func(eventType, targetType string) bool

func ExactMatch(eventType, targetType string) bool {
	return eventType == targetType
}

func PrefixMatch(eventType, targetType string) bool {
	return strings.HasPrefix(eventType, targetType)
}

func ExtractEvents(block types.ScrapedBlock, eventType string) (events []types.ParsedEvent, err error) {
	return extractEventsWithMatcher(block, eventType, ExactMatch)
}

func ExtractEventsWithMatcher(block types.ScrapedBlock, eventType string, matcher EventMatcher) (events []types.ParsedEvent, err error) {
	return extractEventsWithMatcher(block, eventType, matcher)
}

// extractEventsWithMatcher is the shared implementation for extracting events
func extractEventsWithMatcher(block types.ScrapedBlock, eventType string, matcher EventMatcher) (events []types.ParsedEvent, err error) {
	events = parseEvents(block.PreBlock, "", eventType)
	events = append(events, parseEvents(block.BeginBlock, "", eventType)...)

	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return events, err
		}
		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))
		txRes := block.TxResults[txIndex]
		events = append(events, parseEventsWithMatcher(txRes.Events, txHash, eventType, matcher)...)
	}

	events = append(events, parseEvents(block.EndBlock, "", eventType)...)

	return events, nil
}

func parseEventsWithMatcher(events []abci.Event, txHash string, eventType string, matcher EventMatcher) (parsed []types.ParsedEvent) {
	for _, event := range events {
		if matcher(event.Type, eventType) {
			parsed = append(parsed, parseEvent(event, txHash))
		}
	}
	return
}

func parseEvents(events []abci.Event, txHash string, eventType string) (parsed []types.ParsedEvent) {
	return parseEventsWithMatcher(events, txHash, eventType, ExactMatch)
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
