package move_nft

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	nft_pair "github.com/initia-labs/rollytics/indexer/collector/nft-pair"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func (sub *MoveNftSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) error {
	sub.mtx.Lock()
	cacheData, ok := sub.cache[block.Height]
	delete(sub.cache, block.Height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	batchSize := sub.cfg.GetDBBatchSize()
	var mintedCols []types.CollectedNftCollection
	mintMap := make(map[string]map[string]interface{})
	transferMap := make(map[string]string)
	mutMap := make(map[string]string)
	burnMap := make(map[string]interface{})
	updateCountMap := make(map[string]interface{})
	events, err := indexerutil.ExtractEvents(block, "move")
	if err != nil {
		return err
	}

	for _, event := range events {
		typeTag, found := event.AttrMap["type_tag"]
		if !found {
			continue
		}
		data, found := event.AttrMap["data"]
		if !found {
			continue
		}
		dataBytes := []byte(data)

		switch typeTag {
		case "0x1::collection::CreateCollectionEvent":
			var event CreateCollectionEvent
			if err := json.Unmarshal(dataBytes, &event); err != nil {
				return err
			}
			creator, err := util.AccAddressFromString(event.Creator)
			if err != nil {
				return err
			}
			mintedCols = append(mintedCols, types.CollectedNftCollection{
				Addr:    event.Collection,
				Height:  block.Height,
				Name:    event.Name,
				Creator: creator.String(),
			})

		case "0x1::collection::MintEvent":
			var event NftMintAndBurnEvent
			if err := json.Unmarshal(dataBytes, &event); err != nil {
				return err
			}
			if _, ok := mintMap[event.Collection]; !ok {
				mintMap[event.Collection] = make(map[string]interface{})
			}
			mintMap[event.Collection][event.Nft] = nil
			delete(burnMap, event.Nft)
			updateCountMap[event.Collection] = nil

		// NOTE: this might not be related to nft transfer event
		case "0x1::object::TransferEvent":
			var event NftTransferEvent
			if err := json.Unmarshal(dataBytes, &event); err != nil {
				return err
			}
			toAddr, err := util.AccAddressFromString(event.To)
			if err != nil {
				return err
			}
			transferMap[event.Object] = toAddr.String()

		case "0x1::nft::MutationEvent":
			var event NftMutationEvent
			if err := json.Unmarshal(dataBytes, &event); err != nil {
				return err
			}
			if event.MutatedFieldName == "uri" {
				mutMap[event.Nft] = event.NewValue
			}

		case "0x1::collection::BurnEvent":
			var event NftMintAndBurnEvent
			if err := json.Unmarshal(dataBytes, &event); err != nil {
				return err
			}
			burnMap[event.Nft] = nil
			delete(mintMap[event.Collection], event.Nft)
			delete(transferMap, event.Nft)
			delete(mutMap, event.Nft)
			updateCountMap[event.Collection] = nil
		}
	}

	// batch insert collections
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize).Error; err != nil {
		return err
	}

	// batch insert nfts
	var mintedNfts []types.CollectedNft
	for collectionAddr, nftMap := range mintMap {
		creator, err := getCollectionCreator(collectionAddr, tx)
		if err != nil {
			return err
		}

		for nftAddr := range nftMap {
			nftResourceRaw, ok := cacheData.NftResources[nftAddr]
			if !ok {
				return fmt.Errorf("move resource not found for nft address %s", nftAddr)
			}
			var nftResource NftResource
			if err := json.Unmarshal([]byte(nftResourceRaw), &nftResource); err != nil {
				return err
			}
			nftResource.Trim()
			mintedNfts = append(mintedNfts, types.CollectedNft{
				CollectionAddr: collectionAddr,
				TokenId:        nftResource.Data.TokenId,
				Addr:           nftAddr,
				Height:         block.Height,
				Owner:          creator,
				Uri:            nftResource.Data.Uri,
			})
		}
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error; err != nil {
		return err
	}

	// update transferred nfts
	for nftAddr, owner := range transferMap {
		if err := tx.Model(&types.CollectedNft{}).
			Where("addr = ?", nftAddr).
			Updates(map[string]interface{}{"height": block.Height, "owner": owner}).Error; err != nil {
			return err
		}
	}

	// update mutated nfts
	for nftAddr, uri := range mutMap {
		if err := tx.Model(&types.CollectedNft{}).
			Where("addr = ?", nftAddr).
			Updates(map[string]interface{}{"height": block.Height, "uri": uri}).Error; err != nil {
			return err
		}
	}

	// batch delete burned nfts
	var burnedNfts []string
	for nftAddr := range burnMap {
		burnedNfts = append(burnedNfts, nftAddr)
	}
	if err := tx.
		Where("addr IN ?", burnedNfts).
		Delete(&types.CollectedNft{}).Error; err != nil {
		return err
	}

	// update nft count
	for collectionAddr := range updateCountMap {
		var nftCount int64
		if err := tx.Model(&types.CollectedNft{}).
			Where("collection_addr = ?", collectionAddr).
			Count(&nftCount).Error; err != nil {
			return err
		}
		if err := tx.Model(&types.CollectedNftCollection{}).
			Where("addr = ?", collectionAddr).
			Updates(map[string]interface{}{"nft_count": nftCount}).Error; err != nil {
			return err
		}
	}

	return nft_pair.Collect(block, sub.cfg, tx)
}
