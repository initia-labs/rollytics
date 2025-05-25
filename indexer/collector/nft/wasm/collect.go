package wasm

import (
	"encoding/base64"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Collect(block indexertypes.ScrappedBlock, cfg *config.Config, tx *gorm.DB) (err error) {
	batchSize := cfg.GetDBConfig().BatchSize
	mintColMap := make(map[string]types.CollectedNftCollection)
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
		case "instantiate":
			name, found := event.Attributes["collection_name"]
			if !found {
				continue
			}
			creator, found := event.Attributes["collection_creator"]
			if !found {
				continue
			}

			mintColMap[collectionAddr] = types.CollectedNftCollection{
				ChainId: block.ChainId,
				Addr:    collectionAddr,
				Height:  block.Height,
				Name:    name,
				Creator: creator,
			}

		case "mint":
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}
			owner, found := event.Attributes["owner"]
			if !found {
				continue
			}
			uri, found := event.Attributes["token_uri"]
			if !found {
				continue
			}

			if _, ok := mintNftMap[collectionAddr]; !ok {
				mintNftMap[collectionAddr] = make(map[string]types.CollectedNft)
			}
			mintNftMap[collectionAddr][tokenId] = types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
				Height:         block.Height,
				Owner:          owner,
				Uri:            uri,
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
	for _, col := range mintColMap {
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

	return nil
}

func extractEvents(block indexertypes.ScrappedBlock) (events []indexertypes.ParsedEvent, err error) {
	events = parseEvents(block.BeginBlock, "")

	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return events, err
		}
		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))
		txRes := block.TxResults[txIndex]
		events = append(events, parseEvents(txRes.Events, txHash)...)
	}

	events = append(events, parseEvents(block.EndBlock, "")...)

	return events, nil
}

func parseEvents(evts []abci.Event, txHash string) (parsedEvts []indexertypes.ParsedEvent) {
	for _, evt := range evts {
		parsedEvts = append(parsedEvts, parseEvent(evt, txHash))
	}

	return
}

func parseEvent(evt abci.Event, txHash string) indexertypes.ParsedEvent {
	attributes := make(map[string]string)
	for _, attr := range evt.Attributes {
		attributes[attr.Key] = attr.Value
	}
	return indexertypes.ParsedEvent{
		TxHash:     txHash,
		Type:       evt.Type,
		Attributes: attributes,
	}
}
