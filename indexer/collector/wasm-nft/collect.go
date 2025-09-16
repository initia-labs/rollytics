package wasm_nft

import (
	"errors"

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
	mintMap := make(map[util.NftKey]map[string]string)
	transferMap := make(map[util.NftKey]string)
	burnMap := make(map[util.NftKey]interface{})
	updateCountMap := make(map[string]interface{})
	nftTxMap := make(map[string]map[string]map[string]interface{})

	events, err := indexerutil.ExtractEventsWithMatcher(block, CustomContractEventPrefix, WasmEventMatcher)
	if err != nil {
		return err
	}

	for _, event := range events {
		collectionAddr, found := event.AttrMap["_contract_address"]
		if !found {
			continue
		}
		// use hex address for collection instead of acc address
		collectionAddrBytes, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			return err
		}
		collectionAddr = util.BytesToHexWithPrefix(collectionAddrBytes)
		action, found := event.AttrMap["action"]
		if !found {
			continue
		}

		switch action {
		case EventAttrNftMint:
			tokenId, found := event.AttrMap["token_id"]
			if !found {
				continue
			}
			owner, found := event.AttrMap["owner"]
			if !found {
				continue
			}

			nftKey := util.NftKey{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			}

			mintMap[nftKey] = map[string]string{
				"owner": owner,
				"uri":   event.AttrMap["token_uri"],
			}
			delete(burnMap, nftKey)
			updateCountMap[collectionAddr] = nil

			if _, ok := nftTxMap[event.TxHash]; !ok {
				nftTxMap[event.TxHash] = make(map[string]map[string]interface{})
			}
			if _, ok := nftTxMap[event.TxHash][collectionAddr]; !ok {
				nftTxMap[event.TxHash][collectionAddr] = make(map[string]interface{})
			}
			nftTxMap[event.TxHash][collectionAddr][tokenId] = nil
		case EventAttrNftTransfer, EventAttrNftSend:
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
		case EventAttrNftBurn:
			tokenId, found := event.AttrMap["token_id"]
			if !found {
				continue
			}

			nftKey := util.NftKey{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			}
			burnMap[nftKey] = nil

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
	for _, colInfo := range cacheData.ColInfos {
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

	var createdCols []types.CollectedNftCollection
	for collectionAddr, colInfo := range cacheData.ColInfos {
		addrBytes, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			return err
		}

		creatorId := accountIdMap[colInfo.Creator]

		createdCols = append(createdCols, types.CollectedNftCollection{
			Addr:      addrBytes,
			Height:    block.Height,
			Timestamp: block.Timestamp,
			Name:      colInfo.Name,
			CreatorId: creatorId,
		})
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(createdCols, batchSize).Error; err != nil {
		return err
	}

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
			Timestamp:      block.Timestamp,
			OwnerId:        ownerId,
			Uri:            data["uri"],
		})
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error; err != nil {
		return err
	}

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
			Timestamp:      block.Timestamp,
			OwnerId:        ownerId,
		})
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection_addr"}, {Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "timestamp", "owner_id"}),
	}).CreateInBatches(transferredNfts, batchSize).Error; err != nil {
		return err
	}

	burnedNftMap := make(map[string][]string) // collectionAddr -> tokenIds
	for nftKey := range burnMap {
		burnedNftMap[nftKey.CollectionAddr] = append(burnedNftMap[nftKey.CollectionAddr], nftKey.TokenId)
	}

	for collectionAddr, tokenIds := range burnedNftMap {
		collectionAddrBytes, err := util.AccAddressFromString(collectionAddr)
		if err != nil {
			return err
		}
		if err := tx.
			Where("collection_addr = ? AND token_id IN ?", collectionAddrBytes, tokenIds).
			Delete(&types.CollectedNft{}).Error; err != nil {
			return err
		}
	}

	var updateAddrs [][]byte
	for collectionAddr := range updateCountMap {
		collectionAddrBytes, err := util.HexToBytes(collectionAddr)
		if err != nil {
			return err
		}
		updateAddrs = append(updateAddrs, collectionAddrBytes)
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

		txHashBytes, err := util.HexToBytes(txHash)
		if err != nil {
			sub.logger.Error("Failed to decode tx hash", "txHash", txHash, "error", err)
			continue
		}

		if err := tx.Model(&types.CollectedTx{}).
			Where("hash = ?", txHashBytes).
			Update("nft_ids", pq.Array(nftIds)).Error; err != nil {
			return err
		}
	}

	return nft_pair.Collect(block, sub.cfg, tx)
}
