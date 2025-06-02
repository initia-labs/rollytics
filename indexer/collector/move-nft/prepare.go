package move_nft

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"golang.org/x/sync/errgroup"
)

func (sub *MoveNftSubmodule) prepare(block indexertypes.ScrappedBlock) (err error) {
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

	sub.mtx.Lock()
	sub.dataMap[block.Height] = CacheData{
		CollectionMap: collectionMap,
		NftMap:        nftMap,
	}
	sub.mtx.Unlock()

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
			var event NftMintAndBurnEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return colAddrs, nftAddrs, err
			}
			collectionAddrMap[event.Collection] = nil
			nftAddrMap[event.Nft] = nil
		case "0x1::collection::BurnEvent":
			var event NftMintAndBurnEvent
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
