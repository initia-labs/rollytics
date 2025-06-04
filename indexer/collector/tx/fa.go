package tx

import (
	"encoding/json"

	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func collectFa(block indexertypes.ScrappedBlock, cfg *config.Config, tx *gorm.DB) (err error) {
	if cfg.GetVmType() != types.MoveVM {
		return nil
	}

	batchSize := cfg.GetDBBatchSize()
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
