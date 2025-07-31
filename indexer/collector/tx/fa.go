package tx

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func collectFA(block indexertypes.ScrapedBlock, cfg *config.Config, tx *gorm.DB) error {
	if cfg.GetVmType() != types.MoveVM {
		return nil
	}

	batchSize := cfg.GetDBBatchSize()
	var stores []types.CollectedFAStore
	events, err := indexerutil.ExtractEvents(block, "move")
	if err != nil {
		return err
	}

	for _, event := range events {
		typeTag, found := event.AttrMap["type_tag"]
		if !found || typeTag != "0x1::primary_fungible_store::PrimaryStoreCreatedEvent" {
			continue
		}
		data, found := event.AttrMap["data"]
		if !found {
			continue
		}

		var event PrimaryStoreCreatedEvent
		if err = json.Unmarshal([]byte(data), &event); err != nil {
			return err
		}

		storeAddrBytes, err := util.HexToBytes(event.StoreAddr)
		if err != nil {
			return err
		}
		ownerAddrBytes, err := util.HexToBytes(event.OwnerAddr)
		if err != nil {
			return err
		}
		stores = append(stores, types.CollectedFAStore{
			StoreAddr: storeAddrBytes,
			Owner:     ownerAddrBytes,
		})
	}

	// insert fa stores
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(stores, batchSize).Error; err != nil {
		return err
	}

	return nil
}
