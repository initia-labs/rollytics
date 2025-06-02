package wasm_nft

import (
	"errors"
	"fmt"

	"github.com/initia-labs/rollytics/indexer/collector/pair"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (sub *WasmNftSubmodule) collect(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	sub.mtx.Lock()
	cacheData, ok := sub.dataMap[block.Height]
	delete(sub.dataMap, block.Height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	batchSize := sub.cfg.GetDBConfig().BatchSize
	mintColMap := make(map[string]interface{})
	mintNftMap := make(map[string]map[string]types.CollectedNft)
	transferMap := make(map[string]map[string]types.CollectedNft)
	burnMap := make(map[string]map[string]interface{})
	updateCountMap := make(map[string]interface{})
	nftTxMap := make(map[string]map[string]map[string]interface{})

	events, err := extractEvents(block)
	if err != nil {
		return err
	}

	for _, event := range events {
		if event.Type != "wasm" {
			continue
		}

		collectionAddr, found := event.Attributes["_contract_address"]
		if !found {
			continue
		}
		action, found := event.Attributes["action"]
		if !found {
			continue
		}

		switch action {
		case "mint":
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}
			owner, found := event.Attributes["owner"]
			if !found {
				continue
			}

			mintColMap[collectionAddr] = nil
			if _, ok := mintNftMap[collectionAddr]; !ok {
				mintNftMap[collectionAddr] = make(map[string]types.CollectedNft)
			}
			mintNftMap[collectionAddr][tokenId] = types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Height:         block.Height,
				Owner:          owner,
				Uri:            event.Attributes["token_uri"], // might be empty string
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
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}
			recipient, found := event.Attributes["recipient"]
			if !found {
				continue
			}

			if _, ok := transferMap[collectionAddr]; !ok {
				transferMap[collectionAddr] = make(map[string]types.CollectedNft)
			}
			transferMap[collectionAddr][tokenId] = types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Height:         block.Height,
				Owner:          recipient,
			}

			if _, ok := nftTxMap[event.TxHash]; !ok {
				nftTxMap[event.TxHash] = make(map[string]map[string]interface{})
			}
			if _, ok := nftTxMap[event.TxHash][collectionAddr]; !ok {
				nftTxMap[event.TxHash][collectionAddr] = make(map[string]interface{})
			}
			nftTxMap[event.TxHash][collectionAddr][tokenId] = nil
		case "burn":
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}

			if _, ok := burnMap[collectionAddr]; !ok {
				burnMap[collectionAddr] = make(map[string]interface{})
			}
			burnMap[collectionAddr][tokenId] = nil
			delete(mintNftMap[collectionAddr], tokenId)
			delete(transferMap[collectionAddr], tokenId)
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

	// batch insert collections
	var mintedCols []types.CollectedNftCollection
	for collectionAddr := range mintColMap {
		colInfo, ok := cacheData.CollectionMap[collectionAddr]
		if !ok {
			// skip if blacklisted
			if _, found := sub.blacklistMap.Load(collectionAddr); found {
				continue
			}

			return fmt.Errorf("collection info not found for collection address %s", collectionAddr)
		}

		col := types.CollectedNftCollection{
			ChainId: block.ChainId,
			Addr:    collectionAddr,
			Height:  block.Height,
			Name:    colInfo.Name,
			Creator: colInfo.Creator,
		}
		mintedCols = append(mintedCols, col)
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize); res.Error != nil {
		return res.Error
	}

	// batch insert nfts
	var mintedNfts []types.CollectedNft
	for _, nftMap := range mintNftMap {
		for _, nft := range nftMap {
			mintedNfts = append(mintedNfts, nft)
		}
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch update transferred nfts
	var transferredNfts []types.CollectedNft
	for _, nftMap := range transferMap {
		for _, nft := range nftMap {
			transferredNfts = append(transferredNfts, nft)
		}
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain_id"}, {Name: "collection_addr"}, {Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "owner"}),
	}).CreateInBatches(transferredNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch delete burned nfts
	for collectionAddr, nftMap := range burnMap {
		var tokenIds []string
		for tokenId := range nftMap {
			tokenIds = append(tokenIds, tokenId)
		}
		if res := tx.Where("chain_id = ? AND collection_addr = ? AND token_id IN ?", block.ChainId, collectionAddr, tokenIds).Delete(&types.CollectedNft{}); res.Error != nil {
			return res.Error
		}
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

	// batch insert nft txs
	var nftTxs []types.CollectedNftTx
	for txHash, collectionMap := range nftTxMap {
		for collectionAddr, nftMap := range collectionMap {
			for tokenId := range nftMap {
				nftTxs = append(nftTxs, types.CollectedNftTx{
					ChainId:        block.ChainId,
					Hash:           txHash,
					CollectionAddr: collectionAddr,
					TokenId:        tokenId,
					Height:         block.Height,
				})
			}
		}
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(nftTxs, batchSize); res.Error != nil {
		return res.Error
	}

	return pair.Collect(block, sub.cfg, tx)
}
