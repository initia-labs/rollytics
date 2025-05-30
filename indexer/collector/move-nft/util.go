package move_nft

import (
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/initia-labs/rollytics/indexer/types"
)

func extractEvents(block types.ScrappedBlock) []types.ParsedEvent {
	events := parseEvents(block.BeginBlock)

	for _, res := range block.TxResults {
		events = append(events, parseEvents(res.Events)...)
	}

	events = append(events, parseEvents(block.EndBlock)...)

	return events
}

func parseEvents(evts []abci.Event) (parsedEvts []types.ParsedEvent) {
	for _, evt := range evts {
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
