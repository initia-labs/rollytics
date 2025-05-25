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
	mutMap := make(map[string]interface{})
	burnMap := make(map[string]interface{})

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
			delete(burnMap, data.Nft)

		// NOTE: this might not be related to nft transfer event
		case "0x1::object::TransferEvent":
			data := NftTransferEventData{}
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				return err
			}
			transferMap[data.Object] = data.To

		case "0x1::nft::MutationEvent":
			mutation := NftMutationEventData{}
			if err := json.Unmarshal(dataBytes, &mutation); err != nil {
				return err
			}
			mutMap[mutation.Nft] = nil

		case "0x1::collection::BurnEvent":
			burnt := NftMintAndBurnEventData{}
			if err := json.Unmarshal(dataBytes, &burnt); err != nil {
				return err
			}
			burnMap[burnt.Nft] = nil
			delete(mintMap[burnt.Collection], burnt.Nft)
			delete(transferMap, burnt.Nft)
			delete(mutMap, burnt.Nft)
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
			ChainId: block.ChainId,
			Addr:    mintCol,
			Height:  block.Height,
			Name:    colInfo.Data.Name,
			Creator: colInfo.Data.Creator,
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
				Uri:            nftInfo.Data.Uri,
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

	// update transferred nfts
	for transferNft, transferTo := range transferMap {
		if res := tx.Model(&types.CollectedNft{}).Where("chain_id = ? AND addr = ?", block.ChainId, transferNft).Updates(map[string]interface{}{"height": block.Height, "owner": transferTo}); res.Error != nil {
			return res.Error
		}
	}

	// batch update mutated nfts
	var mutatedNfts []types.CollectedNft
	for mutNft := range mutMap {
		nftResource, ok := data.NftMap[mutNft]
		if !ok {
			return fmt.Errorf("move resource not found for nft address %s", mutNft)
		}
		var nftInfo NftData
		if err := json.Unmarshal([]byte(nftResource), &nftInfo); err != nil {
			return err
		}
		nft := types.CollectedNft{
			ChainId: block.ChainId,
			Addr:    mutNft,
			Height:  block.Height,
			Uri:     nftInfo.Data.Uri,
		}
		mutatedNfts = append(mutatedNfts, nft)
	}
	if res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chain_id"}, {Name: "addr"}},
		DoUpdates: clause.AssignmentColumns([]string{"height", "uri"}),
	}).CreateInBatches(mutatedNfts, batchSize); res.Error != nil {
		return res.Error
	}

	// batch delete burned nfts
	var burnedNfts []string
	for burnNft := range burnMap {
		burnedNfts = append(burnedNfts, burnNft)
	}
	if res := tx.Where("chain_id = ? AND addr IN ?", block.ChainId, burnedNfts).Delete(&types.CollectedNft{}); res.Error != nil {
		return res.Error
	}

	// TODO: handle NftCount

	return nil
}

func filterMoveData(block indexertypes.ScrappedBlock) (colAddrs []string, nftAddrs []string, err error) {
	collectionAddrMap := make(map[string]interface{})
	nftAddrMap := make(map[string]interface{})
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

		switch typeTag {
		case "0x1::collection::MintEvent":
			var event NftMintAndBurnEventData
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return colAddrs, nftAddrs, err
			}
			collectionAddrMap[event.Collection] = nil
			nftAddrMap[event.Nft] = nil
		case "0x1::nft::MutationEvent":
			var event NftMutationEventData
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return colAddrs, nftAddrs, err
			}
			nftAddrMap[event.Nft] = nil
		case "0x1::collection::BurnEvent":
			var event NftMintAndBurnEventData
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return colAddrs, nftAddrs, err
			}
			delete(nftAddrMap, event.Nft)
		default:
			continue
		}
	}

	for addr := range collectionAddrMap {
		colAddrs = append(colAddrs, addr)
	}
	for addr := range nftAddrMap {
		nftAddrs = append(nftAddrs, addr)
	}

	return
}
