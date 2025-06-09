package evm_nft

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	nft_pair "github.com/initia-labs/rollytics/indexer/collector/nft-pair"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (sub *EvmNftSubmodule) collect(block indexertypes.ScrapedBlock, tx *gorm.DB) (err error) {
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
	events, err := util.ExtractEvents(block, "evm")
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
		mintedCols = append(mintedCols, types.CollectedNftCollection{
			ChainId: block.ChainId,
			Addr:    collectionAddr,
			Height:  block.Height,
			Name:    name,
		})

		for tokenId, owner := range nftMap {
			tokenUri, ok := cacheData.TokenUris[collectionAddr][tokenId]
			if !ok {
				return fmt.Errorf("token uri info not found for collection address %s and token id %s", collectionAddr, tokenId)
			}
			mintedNfts = append(mintedNfts, types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Height:         block.Height,
				Owner:          owner,
				Uri:            tokenUri,
			})
		}
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedCols, batchSize); res.Error != nil {
		return res.Error
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch update transferred nfts
	var transferredNfts []types.CollectedNft
	for collectionAddr, nftMap := range transferMap {
		for tokenId, owner := range nftMap {
			transferredNfts = append(transferredNfts, types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Owner:          owner,
				Height:         block.Height,
			})
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
			return res.Error
		}
		if res := tx.Model(&types.CollectedNftCollection{}).Where("chain_id = ? AND addr = ?", block.ChainId, collectionAddr).Updates(map[string]interface{}{"nft_count": nftCount}); res.Error != nil {
			return res.Error
		}
	}

	// batch insert nft txs
	var nftTxs []types.CollectedNftTx
	for txHash, collectionMap := range nftTxMap {
		if txHash == "" {
			continue
		}

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

	return nft_pair.Collect(block, sub.cfg, tx)
}
