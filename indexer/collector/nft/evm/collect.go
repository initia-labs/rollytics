package evm

import (
	"encoding/json"
	"fmt"

	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	nfttypes "github.com/initia-labs/rollytics/indexer/collector/nft/types"
	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Collect(block indexertypes.ScrappedBlock, data nfttypes.CacheData, cfg *config.Config, tx *gorm.DB) (err error) {
	batchSize := cfg.GetDBConfig().BatchSize
	mintMap := make(map[string]map[string]string)
	transferMap := make(map[string]map[string]string)
	burnMap := make(map[string]map[string]interface{})

	for _, event := range getEvents(block) {
		if event.Type != "evm" {
			continue
		}

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

			collectionAddr := log.Address
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

			if from == emptyAddr && to != emptyAddr {
				// handle mint
				if _, ok := mintMap[collectionAddr]; !ok {
					mintMap[collectionAddr] = make(map[string]string)
				}
				mintMap[collectionAddr][tokenId] = toAddr.String()
				delete(burnMap[collectionAddr], tokenId)
			} else if from != emptyAddr && to != emptyAddr {
				// handle transfer
				if _, ok := transferMap[collectionAddr]; !ok {
					transferMap[collectionAddr] = make(map[string]string)
				}
				transferMap[collectionAddr][tokenId] = toAddr.String()
			} else if from != emptyAddr && to == emptyAddr {
				// handle burn
				if _, ok := burnMap[collectionAddr]; !ok {
					burnMap[collectionAddr] = make(map[string]interface{})
				}
				burnMap[collectionAddr][tokenId] = nil
				delete(mintMap[collectionAddr], tokenId)
				delete(transferMap[collectionAddr], tokenId)
			}
		}
	}

	// batch insert collections and nfts
	var mintedCols []types.CollectedNftCollection
	var mintedNfts []types.CollectedNft
	for collectionAddr, nftMap := range mintMap {
		name, ok := data.CollectionMap[collectionAddr]
		if !ok {
			return fmt.Errorf("collection name info not found for collection address %s", collectionAddr)
		}
		col := types.CollectedNftCollection{
			ChainId:    block.ChainId,
			Addr:       collectionAddr,
			Height:     block.Height,
			Name:       name,
			OriginName: name,
		}
		mintedCols = append(mintedCols, col)

		for tokenId, owner := range nftMap {
			nftAddr := fmt.Sprintf("%s%s", collectionAddr, tokenId)
			tokenUri, ok := data.NftMap[nftAddr]
			if !ok {
				return fmt.Errorf("token uri info not found for collection address %s and token id %s", collectionAddr, tokenId)
			}
			nft := types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Addr:           nftAddr,
				Height:         block.Height,
				Owner:          owner,
				Uri:            tokenUri,
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

	// batch update transferred nfts
	var transferredNfts []types.CollectedNft
	for collectionAddr, nftMap := range transferMap {
		for tokenId, owner := range nftMap {
			nft := types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Owner:          owner,
				Height:         block.Height,
			}
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

	// TODO: handle NftCount

	return nil
}
