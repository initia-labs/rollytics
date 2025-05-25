package nft

import (
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (sub NftSubmodule) collectWasm(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	batchSize := sub.cfg.GetDBConfig().BatchSize
	mintColMap := make(map[string]types.CollectedNftCollection)
	mintNftMap := make(map[string]map[string]types.CollectedNft)
	transferMap := make(map[string]map[string]types.CollectedNft)
	burnMap := make(map[string]map[string]interface{})

	for _, event := range extractEvents(block) {
		if event.Type != "wasm" {
			continue
		}

		contractAddr, found := event.Attributes["_contract_address"]
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

			mintColMap[contractAddr] = types.CollectedNftCollection{
				ChainId: block.ChainId,
				Addr:    contractAddr,
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

			if _, ok := mintNftMap[contractAddr]; !ok {
				mintNftMap[contractAddr] = make(map[string]types.CollectedNft)
			}
			mintNftMap[contractAddr][tokenId] = types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: contractAddr,
				TokenId:        tokenId,
				Height:         block.Height,
				Owner:          owner,
				Uri:            uri,
			}
			delete(burnMap[contractAddr], tokenId)

		case "transfer_nft", "send_nft":
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}
			recipient, found := event.Attributes["recipient"]
			if !found {
				continue
			}

			if _, ok := transferMap[contractAddr]; !ok {
				transferMap[contractAddr] = make(map[string]types.CollectedNft)
			}
			transferMap[contractAddr][tokenId] = types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: contractAddr,
				TokenId:        tokenId,
				Height:         block.Height,
				Owner:          recipient,
			}

		case "burn":
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}

			if _, ok := burnMap[contractAddr]; !ok {
				burnMap[contractAddr] = make(map[string]interface{})
			}
			burnMap[contractAddr][tokenId] = nil
			delete(mintNftMap[contractAddr], tokenId)
			delete(transferMap[contractAddr], tokenId)
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

	// TODO: handle NftCount

	return nil
}
