package nft

import (
	"encoding/json"
	"errors"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/gofiber/fiber/v2"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	nftTopic  = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	emptyAddr = "0000000000000000000000000000000000000000"
)

func (sub NftSubmodule) prepareEvm(block indexertypes.ScrappedBlock) (err error) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	targetMap, err := filterEvmData(block)
	if err != nil {
		return err
	}

	var collectionAddrs []string
	var queryData []QueryTokenUriData
	for collectionAddr, tokenIdMap := range targetMap {
		collectionAddrs = append(collectionAddrs, collectionAddr)
		for tokenId := range tokenIdMap {
			queryData = append(queryData, QueryTokenUriData{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			})
		}
	}

	var g errgroup.Group
	getCollectionNamesRes := make(chan map[string]string, 1)
	getTokenUrisRes := make(chan map[string]string, 1)

	g.Go(func() error {
		defer close(getCollectionNamesRes)
		nameMap, err := getCollectionNames(collectionAddrs, client, sub.cfg, block.Height)
		if err != nil {
			return err
		}
		getCollectionNamesRes <- nameMap
		return nil
	})

	g.Go(func() error {
		defer close(getTokenUrisRes)
		uriMap, err := getTokenUris(queryData, client, sub.cfg, block.Height)
		if err != nil {
			return err
		}
		getTokenUrisRes <- uriMap
		return nil
	})

	nameMap := <-getCollectionNamesRes
	uriMap := <-getTokenUrisRes
	sub.dataMap[block.Height] = CacheData{
		CollectionMap: nameMap,
		NftMap:        uriMap,
	}

	return nil
}

func (sub NftSubmodule) collectEvm(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	batchSize := sub.cfg.GetDBConfig().BatchSize
	data, ok := sub.dataMap[block.Height]
	if !ok {
		return errors.New("data is not prepared")
	}

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
			tokenId, err := convertHexStringToDecString(log.Topics[3])
			if err != nil {
				return err
			}

			if from == emptyAddr && to != emptyAddr {
				// handle mint
				if _, ok := mintMap[collectionAddr]; !ok {
					mintMap[collectionAddr] = make(map[string]string)
				}
				mintMap[collectionAddr][tokenId] = to // TODO: change to bech32
				delete(burnMap[collectionAddr], tokenId)
			} else if from != emptyAddr && to != emptyAddr {
				// handle transfer
				if _, ok := transferMap[collectionAddr]; !ok {
					transferMap[collectionAddr] = make(map[string]string)
				}
				transferMap[collectionAddr][tokenId] = to // TODO: change to bech32
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
	for collectionAddr, mintNftMap := range mintMap {
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

		for tokenId, to := range mintNftMap {
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
				Owner:          to,
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
	for collectionAddr, transferNftMap := range transferMap {
		for tokenId, transferTo := range transferNftMap {
			nft := types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Owner:          transferTo,
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

func filterEvmData(block indexertypes.ScrappedBlock) (targetMap map[string]map[string]interface{}, err error) {
	targetMap = make(map[string]map[string]interface{}) // collection addr -> token id

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
				return targetMap, err
			}

			if !isEvmNftLog(log) {
				continue
			}

			collectionAddr := log.Address
			from := log.Topics[1]
			to := log.Topics[2]
			tokenId, err := convertHexStringToDecString(log.Topics[3])
			if err != nil {
				return targetMap, err
			}

			if from == emptyAddr && to != emptyAddr {
				// handle mint
				if _, ok := targetMap[collectionAddr]; !ok {
					targetMap[collectionAddr] = make(map[string]interface{})
				}
				targetMap[collectionAddr][tokenId] = nil
			} else if from != emptyAddr && to == emptyAddr {
				// handle burn
				delete(targetMap[collectionAddr], tokenId)
			}
		}
	}

	return
}

func getEvents(block indexertypes.ScrappedBlock) []abci.Event {
	events := block.BeginBlock

	for _, res := range block.TxResults {
		events = append(events, res.Events...)
	}

	events = append(events, block.EndBlock...)

	return events
}

func isEvmNftLog(log evmtypes.Log) bool {
	return len(log.Topics) == 4 && log.Topics[0] == nftTopic && log.Data == "0x"
}
