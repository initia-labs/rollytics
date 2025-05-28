package move

import (
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	nfttypes "github.com/initia-labs/rollytics/indexer/collector/nft/types"
	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Collect(block indexertypes.ScrappedBlock, cacheData nfttypes.CacheData, cfg *config.Config, tx *gorm.DB) (err error) {
	batchSize := cfg.GetDBConfig().BatchSize
	mintMap := make(map[string]map[string]interface{})
	transferMap := make(map[string]string)
	mutMap := make(map[string]string)
	burnMap := make(map[string]interface{})
	updateCountMap := make(map[string]interface{})

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
		dataBytes := []byte(data)

		switch typeTag {
		case "0x1::collection::MintEvent":
			data := NftMintAndBurnEventData{}
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				return err
			}
			if _, ok := mintMap[data.Collection]; !ok {
				mintMap[data.Collection] = make(map[string]interface{})
			}
			mintMap[data.Collection][data.Nft] = nil
			delete(burnMap, data.Nft)
			updateCountMap[data.Collection] = nil

		// NOTE: this might not be related to nft transfer event
		case "0x1::object::TransferEvent":
			data := NftTransferEventData{}
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				return err
			}
			toAddr, err := util.AccAddressFromString(data.To)
			if err != nil {
				return err
			}
			transferMap[data.Object] = toAddr.String()

		case "0x1::nft::MutationEvent":
			mutation := NftMutationEventData{}
			if err := json.Unmarshal(dataBytes, &mutation); err != nil {
				return err
			}
			if mutation.MutatedFieldName == "uri" {
				mutMap[mutation.Nft] = mutation.NewValue
			}

		case "0x1::collection::BurnEvent":
			burnt := NftMintAndBurnEventData{}
			if err := json.Unmarshal(dataBytes, &burnt); err != nil {
				return err
			}
			burnMap[burnt.Nft] = nil
			delete(mintMap[burnt.Collection], burnt.Nft)
			delete(transferMap, burnt.Nft)
			delete(mutMap, burnt.Nft)
			updateCountMap[burnt.Collection] = nil
		}
	}

	// batch insert collections and nfts
	var mintedCols []types.CollectedNftCollection
	var mintedNfts []types.CollectedNft
	for collectionAddr, nftMap := range mintMap {
		colResource, ok := cacheData.CollectionMap[collectionAddr]
		if !ok {
			return fmt.Errorf("move resource not found for collection address %s", collectionAddr)
		}
		var colInfo NftCollectionData
		if err := json.Unmarshal([]byte(colResource), &colInfo); err != nil {
			return err
		}
		creator, err := util.AccAddressFromString(colInfo.Data.Creator)
		if err != nil {
			return err
		}
		col := types.CollectedNftCollection{
			ChainId: block.ChainId,
			Addr:    collectionAddr,
			Height:  block.Height,
			Name:    colInfo.Data.Name,
			Creator: creator.String(),
		}
		mintedCols = append(mintedCols, col)

		for nftAddr := range nftMap {
			nftResource, ok := cacheData.NftMap[nftAddr]
			if !ok {
				return fmt.Errorf("move resource not found for nft address %s", nftAddr)
			}
			var nftInfo NftData
			if err := json.Unmarshal([]byte(nftResource), &nftInfo); err != nil {
				return err
			}
			nftInfo.Trim()
			nft := types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        nftInfo.Data.TokenId,
				Addr:           nftAddr,
				Height:         block.Height,
				Owner:          creator.String(),
				Uri:            nftInfo.Data.Uri,
			}
			mintedNfts = append(mintedNfts, nft)
		}
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize); res.Error != nil {
		return res.Error
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// update transferred nfts
	for nftAddr, owner := range transferMap {
		if res := tx.Model(&types.CollectedNft{}).Where("chain_id = ? AND addr = ?", block.ChainId, nftAddr).Updates(map[string]interface{}{"height": block.Height, "owner": owner}); res.Error != nil {
			return res.Error
		}
	}

	// batch update mutated nfts
	var mutatedNfts []types.CollectedNft
	for nftAddr, uri := range mutMap {
		nft := types.CollectedNft{
			ChainId: block.ChainId,
			Addr:    nftAddr,
			Height:  block.Height,
			Uri:     uri,
		}
		mutatedNfts = append(mutatedNfts, nft)
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain_id"}, {Name: "addr"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "uri"}),
	}).CreateInBatches(mutatedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch delete burned nfts
	var burnedNfts []string
	for nftAddr := range burnMap {
		burnedNfts = append(burnedNfts, nftAddr)
	}
	if res := tx.Where("chain_id = ? AND addr IN ?", block.ChainId, burnedNfts).Delete(&types.CollectedNft{}); res.Error != nil {
		return res.Error
	}

	// update nft count
	for collectionAddr := range updateCountMap {
		var nftCount int64
		if res := tx.Model(&types.CollectedNft{}).Where("chain_id = ? AND collection_addr = ?", block.ChainId, collectionAddr).Count(&nftCount); res.Error != nil {
			return err
		}
		if res := tx.Model(&types.CollectedNftCollection{}).Where("chain_id = ? AND addr = ?", block.ChainId, collectionAddr).Updates(map[string]interface{}{"nft_count": nftCount}); res.Error != nil {
			return err
		}
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
