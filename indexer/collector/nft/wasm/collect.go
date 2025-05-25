package wasm

import (
	abci "github.com/cometbft/cometbft/abci/types"
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

	for _, event := range extractEvents(block) {
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

	return nil
}

func extractEvents(block indexertypes.ScrappedBlock) []indexertypes.ParsedEvent {
	events := parseEvents(block.BeginBlock)

	for _, res := range block.TxResults {
		events = append(events, parseEvents(res.Events)...)
	}

	events = append(events, parseEvents(block.EndBlock)...)

	return events
}

func parseEvents(evts []abci.Event) (parsedEvts []indexertypes.ParsedEvent) {
	for _, evt := range evts {
		parsedEvts = append(parsedEvts, parseEvent(evt))
	}

	return
}

func parseEvent(evt abci.Event) indexertypes.ParsedEvent {
	attributes := make(map[string]string)
	for _, attr := range evt.Attributes {
		attributes[attr.Key] = attr.Value
	}
	return indexertypes.ParsedEvent{
		Type:       evt.Type,
		Attributes: attributes,
	}
}
