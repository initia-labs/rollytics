package move_nft

import (
	"encoding/json"
	"errors"
	"fmt"

	nft_pair "github.com/initia-labs/rollytics/indexer/collector/nft-pair"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
)

func (sub *MoveNftSubmodule) collect(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	sub.mtx.Lock()
	cacheData, ok := sub.cacheMap[block.Height]
	delete(sub.cacheMap, block.Height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	batchSize := sub.cfg.GetDBBatchSize()
	mintMap := make(map[string]map[string]interface{})
	transferMap := make(map[string]string)
	mutMap := make(map[string]string)
	burnMap := make(map[string]interface{})
	updateCountMap := make(map[string]interface{})
	events, err := util.ExtractEvents(block, "move")
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

	// batch insert collections and nfts
	var mintedCols []types.CollectedNftCollection
	var mintedNfts []types.CollectedNft
	for collectionAddr, nftMap := range mintMap {
		colResourceRaw, ok := cacheData.ColResources[collectionAddr]
		if !ok {
			return fmt.Errorf("move resource not found for collection address %s", collectionAddr)
		}
		var colResource CollectionResource
		if err := json.Unmarshal([]byte(colResourceRaw), &colResource); err != nil {
			return err
		}
		creator, err := util.AccAddressFromString(colResource.Data.Creator)
		if err != nil {
			return err
		}
		mintedCols = append(mintedCols, types.CollectedNftCollection{
			ChainId: block.ChainId,
			Addr:    collectionAddr,
			Height:  block.Height,
			Name:    colResource.Data.Name,
			Creator: creator.String(),
		})

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
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        nftResource.Data.TokenId,
				Addr:           nftAddr,
				Height:         block.Height,
				Owner:          creator.String(),
				Uri:            nftResource.Data.Uri,
			})
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

	// update mutated nfts
	for nftAddr, uri := range mutMap {
		if res := tx.Model(&types.CollectedNft{}).Where("chain_id = ? AND addr = ?", block.ChainId, nftAddr).Updates(map[string]interface{}{"height": block.Height, "uri": uri}); res.Error != nil {
			return res.Error
		}
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

	return nft_pair.Collect(block, sub.cfg, tx)
}
