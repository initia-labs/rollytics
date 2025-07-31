package wasm_nft

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lib/pq"

	nft_pair "github.com/initia-labs/rollytics/indexer/collector/nft-pair"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func (sub *WasmNftSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) error {
	sub.mtx.Lock()
	cacheData, ok := sub.cache[block.Height]
	delete(sub.cache, block.Height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	batchSize := sub.cfg.GetDBBatchSize()
	mintColMap := make(map[string]interface{})
	mintMap := make(map[util.NftKey]map[string]string) // NftKey -> {"owner": owner, "uri": uri}
	transferMap := make(map[util.NftKey]string)        // NftKey -> recipient
	burnMap := make(map[string]map[string]interface{})
	updateCountMap := make(map[string]interface{})
	nftTxMap := make(map[string]map[string]map[string]interface{})

	events, err := indexerutil.ExtractEvents(block, "wasm")
	if err != nil {
		return err
	}

	for _, event := range events {
		collectionAddr, found := event.AttrMap["_contract_address"]
		if !found {
			continue
		}
		action, found := event.AttrMap["action"]
		if !found {
			continue
		}

		switch action {
		case "mint":
			tokenId, found := event.AttrMap["token_id"]
			if !found {
				continue
			}
			owner, found := event.AttrMap["owner"]
			if !found {
				continue
			}

			mintColMap[collectionAddr] = nil

			nftKey := util.NftKey{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			}

			mintMap[nftKey] = map[string]string{
				"owner": owner,
				"uri":   event.AttrMap["token_uri"],
			}
			delete(burnMap[collectionAddr], tokenId)
			updateCountMap[collectionAddr] = nil

			if _, ok := nftTxMap[event.TxHash]; !ok {
				nftTxMap[event.TxHash] = make(map[string]map[string]interface{})
			}
			if _, ok := nftTxMap[event.TxHash][collectionAddr]; !ok {
				nftTxMap[event.TxHash][collectionAddr] = make(map[string]interface{})
			}
			nftTxMap[event.TxHash][collectionAddr][tokenId] = nil
		case "transfer_nft", "send_nft":
			tokenId, found := event.AttrMap["token_id"]
			if !found {
				continue
			}
			recipient, found := event.AttrMap["recipient"]
			if !found {
				continue
			}

			nftKey := util.NftKey{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			}
			transferMap[nftKey] = recipient

			if _, ok := nftTxMap[event.TxHash]; !ok {
				nftTxMap[event.TxHash] = make(map[string]map[string]interface{})
			}
			if _, ok := nftTxMap[event.TxHash][collectionAddr]; !ok {
				nftTxMap[event.TxHash][collectionAddr] = make(map[string]interface{})
			}
			nftTxMap[event.TxHash][collectionAddr][tokenId] = nil
		case "burn":
			tokenId, found := event.AttrMap["token_id"]
			if !found {
				continue
			}

			if _, ok := burnMap[collectionAddr]; !ok {
				burnMap[collectionAddr] = make(map[string]interface{})
			}
			burnMap[collectionAddr][tokenId] = nil

			nftKey := util.NftKey{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			}
			delete(mintMap, nftKey)
			delete(transferMap, nftKey)
			updateCountMap[collectionAddr] = nil

			if _, ok := nftTxMap[event.TxHash]; !ok {
				nftTxMap[event.TxHash] = make(map[string]map[string]interface{})
			}
			if _, ok := nftTxMap[event.TxHash][collectionAddr]; !ok {
				nftTxMap[event.TxHash][collectionAddr] = make(map[string]interface{})
			}
			nftTxMap[event.TxHash][collectionAddr][tokenId] = nil
		}
	}

	var allAddresses []string
	creatorAddrs := make(map[string]string) // collectionAddr -> creator

	for collectionAddr := range mintColMap {
		colInfo, ok := cacheData.ColInfos[collectionAddr]
		if !ok {
			if sub.IsBlacklisted(collectionAddr) {
				continue
			}
			return fmt.Errorf("collection info not found for collection address %s", collectionAddr)
		}
		creatorAddrs[collectionAddr] = colInfo.Creator
		allAddresses = append(allAddresses, colInfo.Creator)
	}

	for _, data := range mintMap {
		if owner, ok := data["owner"]; ok {
			allAddresses = append(allAddresses, owner)
		}
	}

	for _, recipient := range transferMap {
		allAddresses = append(allAddresses, recipient)
	}

	accountIdMap, err := util.GetOrCreateAccountIds(tx, allAddresses, true)
	if err != nil {
		return err
	}

	var mintedCols []types.CollectedNftCollection
	for collectionAddr := range mintColMap {
		colInfo, ok := cacheData.ColInfos[collectionAddr]
		if !ok {
			if sub.IsBlacklisted(collectionAddr) {
				continue
			}
			continue // Already checked above
		}
		addrBytes, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			return err
		}

		creatorId := accountIdMap[colInfo.Creator]

		mintedCols = append(mintedCols, types.CollectedNftCollection{
			Addr:      addrBytes,
			Height:    block.Height,
			Name:      colInfo.Name,
			CreatorId: creatorId,
		})
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize).Error; err != nil {
		return err
	}

	// batch insert nfts
	var mintedNfts []types.CollectedNft
	for nftKey, data := range mintMap {
		collectionAddr, err := util.AccAddressFromString(nftKey.CollectionAddr)
		if err != nil {
			return err
		}

		owner := data["owner"]
		ownerId := accountIdMap[owner]

		mintedNfts = append(mintedNfts, types.CollectedNft{
			CollectionAddr: collectionAddr,
			TokenId:        nftKey.TokenId,
			Height:         block.Height,
			OwnerId:        ownerId,
			Uri:            data["uri"],
		})
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error; err != nil {
		return err
	}

	// batch update transferred nfts
	var transferredNfts []types.CollectedNft
	for nftKey, recipient := range transferMap {
		collectionAddr, err := util.AccAddressFromString(nftKey.CollectionAddr)
		if err != nil {
			return err
		}

		ownerId := accountIdMap[recipient]

		transferredNfts = append(transferredNfts, types.CollectedNft{
			CollectionAddr: collectionAddr,
			TokenId:        nftKey.TokenId,
			Height:         block.Height,
			OwnerId:        ownerId,
		})
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection_addr"}, {Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "owner_id"}),
	}).CreateInBatches(transferredNfts, batchSize).Error; err != nil {
		return err
	}

	// batch delete burned nfts
	for collectionAddr, nftMap := range burnMap {
		var tokenIds []string
		for tokenId := range nftMap {
			tokenIds = append(tokenIds, tokenId)
		}
		if err := tx.
			Where("collection_addr = ? AND token_id IN ?", collectionAddr, tokenIds).
			Delete(&types.CollectedNft{}).Error; err != nil {
			return err
		}
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

	// update nft ids to tx table
	for txHash, collectionMap := range nftTxMap {
		if txHash == "" {
			continue
		}

		var keys []util.NftKey
		for collectionAddr, nftMap := range collectionMap {
			for tokenId := range nftMap {
				key := util.NftKey{CollectionAddr: collectionAddr, TokenId: tokenId}
				keys = append(keys, key)
			}
		}

		nftIdMap, err := util.GetOrCreateNftIds(tx, keys, true)
		if err != nil {
			return err
		}

		var nftIds []int64
		for _, key := range keys {
			if id, ok := nftIdMap[key]; ok {
				nftIds = append(nftIds, id)
			}
		}

		if err := tx.Model(&types.CollectedTx{}).
			Where("hash = ?", txHash).
			Update("nft_ids", pq.Array(nftIds)).Error; err != nil {
			return err
		}
	}

	return nft_pair.Collect(block, sub.cfg, tx)
}
