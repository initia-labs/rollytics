package fa

import (
	"encoding/json"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func Collect(block indexertypes.ScrappedBlock, cfg *config.Config, tx *gorm.DB) (err error) {
	if cfg.GetChainConfig().VmType != types.MoveVM {
		return nil
	}

	batchSize := cfg.GetDBConfig().BatchSize
	var stores []types.CollectedFAStore
	for _, event := range extractEvents(block) {
		if event.Type != "move" {
			continue
		}

		typeTag, found := event.Attributes["type_tag"]
		if !found || typeTag != "0x1::primary_fungible_store::PrimaryStoreCreatedEvent" {
			continue
		}
		data, found := event.Attributes["data"]
		if !found {
			continue
		}

		var event PrimaryStoreCreatedEvent
		if err = json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}

		stores = append(stores, types.CollectedFAStore{
			ChainId:   block.ChainId,
			StoreAddr: event.StoreAddr,
			Owner:     event.OwnerAddr,
		})
	}

	// insert fa stores
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(stores, batchSize); res.Error != nil {
		return res.Error
	}

	return nil
}

func extractEvents(block indexertypes.ScrappedBlock) []indexertypes.ParsedEvent {
	events := parseEvents(block.BeginBlock)

	for _, res := range block.TxResults {
		events = append(events, parseEvents(res.Events)...)
	}

	events = append(events, parseEvents(block.EndBlock)...)

	return events
}

func parseEvents(evts []abci.Event) (parsedEvts []indexertypes.ParsedEvent) {
	for _, evt := range evts {
		parsedEvts = append(parsedEvts, parseEvent(evt))
	}

	return
}

func parseEvent(evt abci.Event) indexertypes.ParsedEvent {
	attributes := make(map[string]string)
	for _, attr := range evt.Attributes {
		attributes[attr.Key] = attr.Value
	}
	return indexertypes.ParsedEvent{
		Type:       evt.Type,
		Attributes: attributes,
	}
}
