package wasm_nft

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
)

var parseErrRegex = regexp.MustCompile(`Error parsing into type [^:]+::msg::QueryMsg: unknown variant`)

func (sub *WasmNftSubmodule) prepare(block indexertypes.ScrapedBlock) error {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	colAddrs, err := filterWasmData(block)
	if err != nil {
		return err
	}

	colInfos := make(map[string]CollectionInfo) // collection addr -> collection info

	var g errgroup.Group
	var mtx sync.Mutex
	for _, collectionAddr := range colAddrs {
		addr := collectionAddr
		// skip if blacklisted
		if sub.IsBlacklisted(addr) {
			continue
		}

		g.Go(func() error {
			info, err := getCollectionInfo(addr, client, sub.cfg, block.Height)
			if err != nil {
				errString := fmt.Sprintf("%+v", err)
				if parseErrRegex.MatchString(errString) {
					sub.AddToBlacklist(addr)
					return nil
				}

				return err
			}

			mtx.Lock()
			colInfos[addr] = info
			mtx.Unlock()

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	sub.mtx.Lock()
	sub.cache[block.Height] = CacheData{
		ColInfos: colInfos,
	}
	sub.mtx.Unlock()

	return nil
}

func filterWasmData(block indexertypes.ScrapedBlock) (colAddrs []string, err error) {
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
