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

	creators, err := filterWasmData(block)
	if err != nil {
		return err
	}

	colInfos := make(map[string]CollectionInfo)
	var g errgroup.Group
	var mtx sync.Mutex

	for collectionAddr, creator := range creators {
		addr := collectionAddr
		creatorAddr := creator

		if sub.IsBlacklisted(addr) {
			continue
		}

		g.Go(func() error {
			name, err := getCollectionName(addr, client, sub.cfg, block.Height)
			if err != nil {
				errString := fmt.Sprintf("%+v", err)
				if parseErrRegex.MatchString(errString) {
					sub.AddToBlacklist(addr)
					return nil
				}
				return err
			}

			mtx.Lock()
			colInfos[addr] = CollectionInfo{
				Name:    name,
				Creator: creatorAddr,
			}
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

func filterWasmData(block indexertypes.ScrapedBlock) (creators map[string]string, err error) {
	creators = make(map[string]string) // collectionAddr -> creator
	events, err := util.ExtractEvents(block, "wasm")
	if err != nil {
		return creators, err
	}

	for _, event := range events {
		collectionAddr, found := event.AttrMap["_contract_address"]
		if !found {
			continue
		}

		creator, hasCreator := event.AttrMap["creator"]
		_, hasMinter := event.AttrMap["minter"]
		if hasCreator && hasMinter {
			creators[collectionAddr] = creator
		}
	}

	return creators, err
}
