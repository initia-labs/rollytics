package evm_nft

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	nft_pair "github.com/initia-labs/rollytics/indexer/collector/nft-pair"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	indexerutil "github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func (sub *EvmNftSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) error {
	sub.mtx.Lock()
	cacheData, ok := sub.cache[block.Height]
	delete(sub.cache, block.Height)
	sub.mtx.Unlock()

	if !ok {
		return errors.New("data is not prepared")
	}

	batchSize := sub.cfg.GetDBBatchSize()
	mintMap := make(map[string]map[string]string)
	transferMap := make(map[string]map[string]string)
	burnMap := make(map[string]map[string]interface{})
	updateCountMap := make(map[string]interface{})
	nftTxMap := make(map[string]map[string]map[string]interface{})
	events, err := indexerutil.ExtractEvents(block, "evm")
	if err != nil {
		return err
	}

	for _, event := range events {
		for _, attr := range event.Attributes {
			if attr.Key != "log" {
				continue
			}

			var log evmtypes.Log
			if err := json.Unmarshal([]byte(attr.Value), &log); err != nil {
				return err
			}

			if !isEvmNftLog(log) {
				continue
			}

			collectionAddr := strings.ToLower(log.Address)
			from := log.Topics[1]
			to := log.Topics[2]
			toAddr, err := util.AccAddressFromString(to)
			if err != nil {
				return err
			}
			tokenId, err := convertHexStringToDecString(log.Topics[3])
			if err != nil {
				return err
			}

			switch {
			case from == emptyAddr && to != emptyAddr:
				// handle mint
				if _, ok := mintMap[collectionAddr]; !ok {
					mintMap[collectionAddr] = make(map[string]string)
				}
				mintMap[collectionAddr][tokenId] = toAddr.String()
				delete(burnMap[collectionAddr], tokenId)
				updateCountMap[collectionAddr] = nil
			case from != emptyAddr && to != emptyAddr:
				// handle transfer
				if _, ok := transferMap[collectionAddr]; !ok {
					transferMap[collectionAddr] = make(map[string]string)
				}
				transferMap[collectionAddr][tokenId] = toAddr.String()
			case from != emptyAddr && to == emptyAddr:
				// handle burn
				if _, ok := burnMap[collectionAddr]; !ok {
					burnMap[collectionAddr] = make(map[string]interface{})
				}
				burnMap[collectionAddr][tokenId] = nil
				delete(mintMap[collectionAddr], tokenId)
				delete(transferMap[collectionAddr], tokenId)
				updateCountMap[collectionAddr] = nil
			default:
				continue
			}

			if _, ok := nftTxMap[event.TxHash]; !ok {
				nftTxMap[event.TxHash] = make(map[string]map[string]interface{})
			}
			if _, ok := nftTxMap[event.TxHash][collectionAddr]; !ok {
				nftTxMap[event.TxHash][collectionAddr] = make(map[string]interface{})
			}
			nftTxMap[event.TxHash][collectionAddr][tokenId] = nil
		}
	}

	// batch insert collections and nfts
	var mintedCols []types.CollectedNftCollection
	var mintedNfts []types.CollectedNft
	for collectionAddr, nftMap := range mintMap {
		name, ok := cacheData.ColNames[collectionAddr]
		if !ok {
			// skip if blacklisted
			if sub.IsBlacklisted(collectionAddr) {
				continue
			}

			return fmt.Errorf("collection name info not found for collection address %s", collectionAddr)
		}

		creator, err := getCollectionCreator(collectionAddr, tx)
		if err != nil {
			return err
		}

		mintedCols = append(mintedCols, types.CollectedNftCollection{
			Addr:    collectionAddr,
			Height:  block.Height,
			Name:    name,
			Creator: creator,
		})

		for tokenId, owner := range nftMap {
			mintedNfts = append(mintedNfts, types.CollectedNft{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Height:         block.Height,
				Owner:          owner,
				Uri:            cacheData.TokenUris[collectionAddr][tokenId],
			})
		}
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize).Error; err != nil {
		return err
	}
	if err := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize).Error; err != nil {
		return err
	}

	// batch update transferred nfts
	var transferredNfts []types.CollectedNft
	for collectionAddr, nftMap := range transferMap {
		for tokenId, owner := range nftMap {
			transferredNfts = append(transferredNfts, types.CollectedNft{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Owner:          owner,
				Height:         block.Height,
			})
		}
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection_addr"}, {Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "owner"}),
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

		nftIds, err := util.GetOrCreateNftIds(tx, keys, true)
		if err != nil {
			return err
		}

		if err := tx.Model(&types.CollectedTx{}).
			Where("hash = ?", txHash).
			Updates(map[string]interface{}{"nft_ids": nftIds}).Error; err != nil {
			return err
		}
	}

	return nft_pair.Collect(block, sub.cfg, tx)
}
