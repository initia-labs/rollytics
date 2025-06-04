package move_nft

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
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
	colResourcesChan := make(chan map[string]string, 1)
	nftResourcesChan := make(chan map[string]string, 1)

	g.Go(func() error {
		defer close(colResourcesChan)
		colResources, err := getCollectionResources(colAddrs, client, sub.cfg, block.Height)
		if err != nil {
			return err
		}
		colResourcesChan <- colResources
		return nil
	})

	g.Go(func() error {
		defer close(nftResourcesChan)
		nftResources, err := getNftResources(nftAddrs, client, sub.cfg, block.Height)
		if err != nil {
			return err
		}
		nftResourcesChan <- nftResources
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	colResources := <-colResourcesChan
	nftResources := <-nftResourcesChan

	sub.mtx.Lock()
	sub.cacheMap[block.Height] = CacheData{
		ColResources: colResources,
		NftResources: nftResources,
	}
	sub.mtx.Unlock()

	return nil
}

func filterMoveData(block indexertypes.ScrappedBlock) (colAddrs []string, nftAddrs []string, err error) {
	collectionAddrMap := make(map[string]interface{})
	nftAddrMap := make(map[string]interface{})
	events, err := util.ExtractEvents(block, "move")
	if err != nil {
		return colAddrs, nftAddrs, err
	}

	for _, event := range events {
		typeTag, found := event.AttrMap["type_tag"]
		if !found {
			continue
		}
		data, found := event.AttrMap["data"]
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
