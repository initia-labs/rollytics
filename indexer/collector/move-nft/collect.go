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
	var collectionEvents []CreateCollectionEvent
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
			collectionEvents = append(collectionEvents, event)

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

	// Collect all addresses (creators and transfer recipients)
	var allAddresses []string
	// mint
	for _, event := range collectionEvents {
		creator, err := util.AccAddressFromString(event.Creator)
		if err != nil {
			return err
		}
		allAddresses = append(allAddresses, creator.String())
	}

	// transfer
	for _, owner := range transferMap {
		allAddresses = append(allAddresses, owner)
	}

	accountIdMap, err := util.GetOrCreateAccountIds(tx, allAddresses, true)
	if err != nil {
		return err
	}

	var mintedCols []types.CollectedNftCollection
	for _, event := range collectionEvents {
		creator, err := util.AccAddressFromString(event.Creator)
		if err != nil {
			return err
		}
		creatorId := accountIdMap[creator.String()]

		collectionAddr, err := util.HexToBytes(event.Collection)
		if err != nil {
			return err
		}
		mintedCols = append(mintedCols, types.CollectedNftCollection{
			Addr:      collectionAddr,
			Height:    block.Height,
			Name:      event.Name,
			CreatorId: creatorId,
		})
	}

	// batch insert collections
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize).Error; err != nil {
		return err
	}

	// batch insert nfts
	var mintedNfts []types.CollectedNft
	for collectionAddr, nftMap := range mintMap {
		creatorId, err := getCollectionCreatorId(collectionAddr, tx)
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
			collectionAddrBytes, err := util.HexToBytes(collectionAddr)
			if err != nil {
				return err
			}
			nftAddrBytes, err := util.HexToBytes(nftAddr)
			if err != nil {
				return err
			}

			mintedNfts = append(mintedNfts, types.CollectedNft{
				CollectionAddr: collectionAddrBytes,
				TokenId:        nftResource.Data.TokenId,
				Addr:           nftAddrBytes,
				Height:         block.Height,
				OwnerId:        creatorId,
				Uri:            nftResource.Data.Uri,
			})
		}
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error; err != nil {
		return err
	}

	// update transferred nfts
	for nftAddr, owner := range transferMap {
		ownerId := accountIdMap[owner]

		if err := tx.Model(&types.CollectedNft{}).
			Where("addr = ?", nftAddr).
			Updates(map[string]interface{}{"height": block.Height, "owner_id": ownerId}).Error; err != nil {
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
	var updateAddrs []string
	for collectionAddr := range updateCountMap {
		updateAddrs = append(updateAddrs, collectionAddr)
	}

	var nftCounts []indexertypes.NftCount
	if err := tx.Table("nft").
		Select("collection_addr, COUNT(*) as count").
		Where("collection_addr IN ?", updateAddrs).
		Group("collection_addr").
		Scan(&nftCounts).Error; err != nil {
		return err
	}

	for _, nftCount := range nftCounts {
		if err := tx.Model(&types.CollectedNftCollection{}).
			Where("addr = ?", nftCount.CollectionAddr).
			Update("nft_count", nftCount.Count).Error; err != nil {
			return err
		}
	}

	return nft_pair.Collect(block, sub.cfg, tx)
}
