package nft

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (sub NftSubmodule) prepareMove(block indexertypes.ScrappedBlock) (err error) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	colAddrs, nftAddrs, err := filterMoveData(block)
	if err != nil {
		return err
	}

	var g errgroup.Group
	getCollectionsRes := make(chan map[string]string, 1)
	getNftsRes := make(chan map[string]string, 1)

	g.Go(func() error {
		defer close(getCollectionsRes)
		collectionMap, err := getCollections(colAddrs, client, sub.cfg, block.Height)
		if err != nil {
			return err
		}
		getCollectionsRes <- collectionMap
		return nil
	})

	g.Go(func() error {
		defer close(getNftsRes)
		nftMap, err := getNfts(nftAddrs, client, sub.cfg, block.Height)
		if err != nil {
			return err
		}
		getNftsRes <- nftMap
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	collectionMap := <-getCollectionsRes
	nftMap := <-getNftsRes
	sub.dataMap[block.Height] = CacheData{
		CollectionMap: collectionMap,
		NftMap:        nftMap,
	}

	return nil
}

func (sub NftSubmodule) collectMove(block indexertypes.ScrappedBlock, tx *gorm.DB) (err error) {
	batchSize := sub.cfg.GetDBConfig().BatchSize
	data, ok := sub.dataMap[block.Height]
	if !ok {
		return errors.New("data is not prepared")
	}

	mintMap := make(map[string]map[string]interface{})
	transferMap := make(map[string]string)
	mutColMap := make(map[string]interface{})
	mutNftMap := make(map[string]interface{})
	burnNftMap := make(map[string]interface{})

	for _, event := range extractEvents(block) {
		if event.Type != "move" {
			continue
		}

		typeTag, found := event.Attributes["type_tag"]
		if !found {
			continue
		}
		data, found := event.Attributes["data"]
		if !found {
			continue
		}
		dataBytes := []byte(data)

		switch typeTag {
		case "0x1::collection::MintEvent":
			data := NftMintAndBurnEventData{}
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				return err
			}
			if _, ok := mintMap[data.Collection]; !ok {
				mintMap[data.Collection] = make(map[string]interface{})
			}
			mintMap[data.Collection][data.Nft] = nil
			delete(burnNftMap, data.Nft)

		case "0x1::object::TransferEvent":
			data := NftTransferEventData{}
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				return err
			}
			transferMap[data.Object] = data.To

		case "0x1::nft::MutationEvent", "0x1::collection::MutationEvent":
			mutation := MutationEventData{}
			if err := json.Unmarshal(dataBytes, &mutation); err != nil {
				return err
			}
			if mutation.Collection != "" {
				mutColMap[mutation.Collection] = nil
			} else if mutation.Nft != "" {
				mutNftMap[mutation.Nft] = nil
			} else {
				return fmt.Errorf("unknown mutation event: %s", mutation)
			}

		case "0x1::collection::BurnEvent":
			burnt := NftMintAndBurnEventData{}
			if err := json.Unmarshal(dataBytes, &burnt); err != nil {
				return err
			}
			burnNftMap[burnt.Nft] = nil
			for _, mintNftMap := range mintMap {
				delete(mintNftMap, burnt.Nft)
			}
			delete(transferMap, burnt.Nft)
			delete(mutNftMap, burnt.Nft)
		}
	}

	// batch insert collections and nfts
	var mintedCols []types.CollectedNftCollection
	var mintedNfts []types.CollectedNft
	for mintCol, mintNftMap := range mintMap {
		colResource, ok := data.CollectionMap[mintCol]
		if !ok {
			return fmt.Errorf("move resource not found for collection address %s", mintCol)
		}
		var colInfo NftCollectionData
		if err := json.Unmarshal([]byte(colResource), &colInfo); err != nil {
			return err
		}
		col := types.CollectedNftCollection{
			ChainId:     block.ChainId,
			Addr:        mintCol,
			Height:      block.Height,
			Name:        colInfo.Data.Name,
			Creator:     colInfo.Data.Creator,
			Description: colInfo.Data.Description,
		}
		mintedCols = append(mintedCols, col)

		for mintNft := range mintNftMap {
			nftResource, ok := data.NftMap[mintNft]
			if !ok {
				return fmt.Errorf("move resource not found for nft address %s", mintNft)
			}
			var nftInfo NftData
			if err := json.Unmarshal([]byte(nftResource), &nftInfo); err != nil {
				return err
			}
			nftInfo.Trim()
			nft := types.CollectedNft{
				ChainId:        block.ChainId,
				CollectionAddr: mintCol,
				TokenId:        nftInfo.Data.TokenId,
				Addr:           mintNft,
				Height:         block.Height,
				Owner:          colInfo.Data.Creator,
				Description:    nftInfo.Data.Description,
				Uri:            nftInfo.Data.Uri,
			}
			mintedNfts = append(mintedNfts, nft)
		}
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain_id"}, {Name: "addr"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "name", "creator", "description"}),
	}).CreateInBatches(mintedCols, batchSize); res.Error != nil {
		return res.Error
	}
	if res := tx.Clauses(orm.DoNothingWhenConflict).CreateInBatches(mintedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch update transferred nfts
	var transferredNfts []types.CollectedNft
	for transferNft, transferTo := range transferMap {
		nft := types.CollectedNft{
			ChainId: block.ChainId,
			Addr:    transferNft,
			Owner:   transferTo,
			Height:  block.Height,
		}
		transferredNfts = append(transferredNfts, nft)
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "addr"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "owner"}),
	}).CreateInBatches(transferredNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch update mutated collections
	var mutatedCols []types.CollectedNftCollection
	for mutCol := range mutColMap {
		colResource, ok := data.CollectionMap[mutCol]
		if !ok {
			return fmt.Errorf("move resource not found for collection address %s", mutCol)
		}
		var colInfo NftCollectionData
		if err := json.Unmarshal([]byte(colResource), &colInfo); err != nil {
			return err
		}
		col := types.CollectedNftCollection{
			ChainId:     block.ChainId,
			Addr:        mutCol,
			Height:      block.Height,
			Description: colInfo.Data.Description,
		}
		mutatedCols = append(mutatedCols, col)
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain_id"}, {Name: "addr"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "description"}),
	}).CreateInBatches(mutatedCols, batchSize); res.Error != nil {
		return res.Error
	}

	// batch update mutated nfts
	var mutatedNfts []types.CollectedNft
	for mutNft := range mutNftMap {
		nftResource, ok := data.NftMap[mutNft]
		if !ok {
			return fmt.Errorf("move resource not found for nft address %s", mutNft)
		}
		var nftInfo NftData
		if err := json.Unmarshal([]byte(nftResource), &nftInfo); err != nil {
			return err
		}
		nft := types.CollectedNft{
			ChainId:     block.ChainId,
			Addr:        mutNft,
			Height:      block.Height,
			Description: nftInfo.Data.Description,
			Uri:         nftInfo.Data.Uri,
		}
		mutatedNfts = append(mutatedNfts, nft)
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "addr"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "description", "uri"}),
	}).CreateInBatches(mutatedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch delete burned nfts
	var burnedNfts []string
	for burnNft := range burnNftMap {
		burnedNfts = append(burnedNfts, burnNft)
	}
	if res := tx.Where("addr IN ?", burnedNfts).Delete(&types.CollectedNft{}); res.Error != nil {
		return res.Error
	}

	// TODO: handle NftCount

	return nil
}
