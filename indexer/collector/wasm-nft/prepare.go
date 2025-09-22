package wasm_nft

import (
	"fmt"
	"regexp"
	"sync"

	"golang.org/x/sync/errgroup"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
)

var (
	EventTypeWasm = "wasm"

	ContractAttrKey = "_contract_address"
	MinterAttrKey   = "minter"
	CreatorAttrKey  = "creator"
	OwnerAttrKey    = "owner"

	parseErrRegex = regexp.MustCompile(`Error parsing into type [^:]+::msg::QueryMsg: unknown variant`)
)

func (sub *WasmNftSubmodule) prepare(block indexertypes.ScrapedBlock) error {
	collectionAddrs := filterCollectionAddrs(block)

	colInfos := make(map[string]CollectionInfo)
	var g errgroup.Group
	var mtx sync.Mutex

	for collectionAddr := range collectionAddrs {
		addr := collectionAddr
		if sub.IsBlacklisted(addr) {
			continue
		}

		g.Go(func() error {
			name, creatorAddr, err := GetCollectionData(addr, sub.cfg, block.Height)
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

	if err := g.Wait(); err != nil {
		return err
	}

	sub.mtx.Lock()
	sub.cache[block.Height] = CacheData{
		ColInfos: colInfos,
	}
	sub.mtx.Unlock()

	return nil
}

func filterCollectionAddrs(block indexertypes.ScrapedBlock) map[string]bool {
	collectionMap := make(map[string]bool)
	events, err := util.ExtractEvents(block, EventTypeWasm)
	if err != nil {
		return collectionMap
	}

	for _, event := range events {
		collectionAddr, ok := event.AttrMap[ContractAttrKey]
		if !ok {
			continue
		}

		if IsValidCollectionEvent(event.AttrMap) {
			collectionMap[collectionAddr] = true
		}
	}

	return collectionMap
}

func IsValidCollectionEvent(attrMap map[string]string) bool {
	_, hasMinter := attrMap[MinterAttrKey]

	_, hasCreator := attrMap[CreatorAttrKey]
	_, hasOwner := attrMap[OwnerAttrKey]

	return hasMinter && (hasCreator || hasOwner)
}
