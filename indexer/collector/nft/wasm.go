package nft

import (
	"fmt"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (sub NftSubmodule) collectWasm(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	batchSize := sub.cfg.GetDBConfig().BatchSize
	mintColMap := make(map[string]types.CollectedNftCollection)
	mintNftMap := make(map[string]types.CollectedNft)
	transferMap := make(map[string]types.CollectedNft)
	burnMap := make(map[string]interface{})

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
			addr := fmt.Sprintf("%s%s", contractAddr, tokenId)

			mintNftMap[addr] = types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: contractAddr,
				TokenId:        tokenId,
				Addr:           addr,
				Height:         block.Height,
				Owner:          owner,
				Uri:            uri,
			}
			delete(burnMap, addr)

		case "transfer_nft", "send_nft":
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}
			recipient, found := event.Attributes["recipient"]
			if !found {
				continue
			}
			addr := fmt.Sprintf("%s%s", contractAddr, tokenId)

			transferMap[addr] = types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: contractAddr,
				TokenId:        tokenId,
				Addr:           addr,
				Height:         block.Height,
				Owner:          recipient,
			}

		case "burn":
			tokenId, found := event.Attributes["token_id"]
			if !found {
				continue
			}
			addr := fmt.Sprintf("%s%s", contractAddr, tokenId)

			burnMap[addr] = nil
			delete(mintNftMap, addr)
			delete(transferMap, addr)
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
	for _, nft := range mintNftMap {
		mintedNfts = append(mintedNfts, nft)
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch update transferred nfts
	var transferredNfts []types.CollectedNft
	for _, nft := range transferMap {
		transferredNfts = append(transferredNfts, nft)
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "addr"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "owner"}),
	}).CreateInBatches(transferredNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch delete burned nfts
	var burnedNfts []string
	for nft := range burnMap {
		burnedNfts = append(burnedNfts, nft)
	}
	if res := tx.Where("addr IN ?", burnedNfts).Delete(&types.CollectedNft{}); res.Error != nil {
		return res.Error
	}

	// TODO: handle NftCount

	return nil
}
