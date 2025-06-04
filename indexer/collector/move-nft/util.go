package move_nft

import (
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/initia-labs/rollytics/indexer/types"
)

func extractEvents(block types.ScrappedBlock, eventType string) []types.ParsedEvent {
	events := parseEvents(block.BeginBlock, eventType)

	for _, res := range block.TxResults {
		events = append(events, parseEvents(res.Events, eventType)...)
	}

	events = append(events, parseEvents(block.EndBlock, eventType)...)

	return events
}

func parseEvents(evts []abci.Event, eventType string) (parsedEvts []types.ParsedEvent) {
	for _, evt := range evts {
		if evt.Type != eventType {
			continue
		}

		parsedEvts = append(parsedEvts, parseEvent(evt))
	}

	return
}

func parseEvent(evt abci.Event) types.ParsedEvent {
	attributes := make(map[string]string)
	for _, attr := range evt.Attributes {
		attributes[attr.Key] = attr.Value
	}
	return types.ParsedEvent{
		Type:       evt.Type,
		Attributes: attributes,
	}
}
