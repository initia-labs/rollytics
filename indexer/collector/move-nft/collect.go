package move_nft

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/initia-labs/rollytics/indexer/collector/pair"
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

	return pair.Collect(block, sub.cfg, tx)
}
