package nft

import (
	"encoding/json"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/initia-labs/rollytics/indexer/types"
)

const maxRetries = 5

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

func filterMoveData(block types.ScrappedBlock) (colAddrs []string, nftAddrs []string, err error) {
	collectionAddrMap := make(map[string]interface{})
	nftAddrMap := make(map[string]interface{})
	for _, event := range extractEvents(block) {
		if event.Type != "move" {
			continue
		}

		typeTag, found := event.Attributes["type_tag"]
		if !found {
			continue
		}
		data, found := event.Attributes["data"]
		if !found {
			continue
		}

		switch typeTag {
		case "0x1::collection::MintEvent":
			var event NftMintAndBurnEventData
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return colAddrs, nftAddrs, err
			}
			collectionAddrMap[event.Collection] = nil
			nftAddrMap[event.Nft] = nil
		case "0x1::nft::MutationEvent", "0x1::collection::MutationEvent":
			var event MutationEventData
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return colAddrs, nftAddrs, err
			}
			if event.Collection != "" {
				collectionAddrMap[event.Collection] = nil
			} else if event.Nft != "" {
				nftAddrMap[event.Nft] = nil
			}
		case "0x1::collection::BurnEvent":
			var event NftMintAndBurnEventData
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return colAddrs, nftAddrs, err
			}
			delete(nftAddrMap, event.Nft)
		default:
			continue
		}
	}

	for addr := range collectionAddrMap {
		colAddrs = append(colAddrs, addr)
	}
	for addr := range nftAddrMap {
		nftAddrs = append(nftAddrs, addr)
	}

	return
}
