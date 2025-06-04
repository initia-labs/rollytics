package wasm_nft

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
	"golang.org/x/sync/errgroup"
)

func (sub *WasmNftSubmodule) prepare(block indexertypes.ScrappedBlock) (err error) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	colAddrs, err := filterWasmData(block)
	if err != nil {
		return err
	}

	infoMap := make(map[string]CacheCollectionInfo) // collection addr -> collection info

	var g errgroup.Group
	var mtx sync.Mutex
	for _, collectionAddr := range colAddrs {
		addr := collectionAddr
		// skip if blacklisted
		if _, found := sub.blacklistMap.Load(addr); found {
			continue
		}

		g.Go(func() error {
			info, err := getCollectionInfo(addr, client, sub.cfg, block.Height)
			if err != nil {
				errString := fmt.Sprintf("%+v", err)
				if strings.Contains(errString, "Error parsing into type sg721_base::msg::QueryMsg: unknown variant") {
					sub.blacklistMap.Store(addr, nil)
					return nil
				}

				return err
			}

			mtx.Lock()
			infoMap[addr] = info
			mtx.Unlock()

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	sub.mtx.Lock()
	sub.cacheMap[block.Height] = CacheData{
		CollectionMap: infoMap,
	}
	sub.mtx.Unlock()

	return nil
}

func filterWasmData(block indexertypes.ScrappedBlock) (colAddrs []string, err error) {
	collectionAddrMap := make(map[string]interface{})
	events, err := util.ExtractEvents(block, "wasm")
	if err != nil {
		return colAddrs, err
	}

	for _, event := range events {
		collectionAddr, found := event.AttrMap["_contract_address"]
		if !found {
			continue
		}
		action, found := event.AttrMap["action"]
		if !found || action != "mint" {
			continue
		}

		collectionAddrMap[collectionAddr] = nil
	}

	for addr := range collectionAddrMap {
		colAddrs = append(colAddrs, addr)
	}

	return
}
