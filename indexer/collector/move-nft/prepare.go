package move_nft

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/fiber/v2"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"golang.org/x/sync/errgroup"
)

const nftStructTag = "0x1::nft::Nft"

func (sub *MoveNftSubmodule) prepare(block indexertypes.ScrapedBlock) (err error) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	nftAddrs, err := filterMoveData(block)
	if err != nil {
		return err
	}

	nftResources := make(map[string]string) // nft addr -> nft resource

	var g errgroup.Group
	var mtx sync.Mutex

	for _, nftAddr := range nftAddrs {
		addr := nftAddr
		g.Go(func() error {
			resource, err := getMoveResource(addr, nftStructTag, client, sub.cfg, block.Height)
			if err != nil {
				return err
			}

			mtx.Lock()
			nftResources[addr] = resource.Resource.MoveResource
			mtx.Unlock()

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	sub.mtx.Lock()
	sub.cache[block.Height] = CacheData{
		NftResources: nftResources,
	}
	sub.mtx.Unlock()

	return nil
}

func filterMoveData(block indexertypes.ScrapedBlock) (nftAddrs []string, err error) {
	nftAddrMap := make(map[string]interface{})
	events, err := util.ExtractEvents(block, "move")
	if err != nil {
		return nftAddrs, err
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
				return nftAddrs, err
			}
			nftAddrMap[event.Nft] = nil
		case "0x1::collection::BurnEvent":
			var event NftMintAndBurnEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				return nftAddrs, err
			}
			delete(nftAddrMap, event.Nft)
		default:
			continue
		}
	}

	for addr := range nftAddrMap {
		nftAddrs = append(nftAddrs, addr)
	}

	return
}
