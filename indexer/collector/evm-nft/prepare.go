package evm_nft

import (
	"encoding/json"
	"strings"
	"sync"

	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"golang.org/x/sync/errgroup"

	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"github.com/initia-labs/rollytics/indexer/util"
)

func (sub *EvmNftSubmodule) prepare(block indexertypes.ScrapedBlock) error {
	targetMap, err := filterEvmData(block)
	if err != nil {
		return err
	}

	colNames := make(map[string]string)             // collection addr -> collection name
	tokenUris := make(map[string]map[string]string) // collection addr -> token id -> token uri

	var g errgroup.Group
	var nameMtx sync.Mutex
	var uriMtx sync.Mutex

	for collectionAddr, tokenIdMap := range targetMap {
		if sub.IsBlacklisted(collectionAddr) {
			continue
		}

		addr := collectionAddr
		g.Go(func() error {
			name, err := getCollectionName(addr, sub.cfg, block.Height)
			if err != nil {
				if isEvmRevertError(err) {
					sub.AddToBlacklist(addr)
					return nil
				}

				return err
			}

			nameMtx.Lock()
			colNames[addr] = name
			nameMtx.Unlock()

			return nil
		})

		for tokenId := range tokenIdMap {
			id := tokenId
			g.Go(func() error {
				tokenUri, err := getTokenUri(addr, id, sub.cfg, block.Height)
				if err != nil {
					if isEvmRevertError(err) {
						return nil
					}

					return err
				}

				uriMtx.Lock()
				if _, ok := tokenUris[addr]; !ok {
					tokenUris[addr] = make(map[string]string)
				}
				tokenUris[addr][id] = tokenUri
				uriMtx.Unlock()

				return nil
			})
		}
	}

	if err = g.Wait(); err != nil {
		return err
	}

	sub.mtx.Lock()
	sub.cache[block.Height] = CacheData{
		ColNames:  colNames,
		TokenUris: tokenUris,
	}
	sub.mtx.Unlock()

	return nil
}

func filterEvmData(block indexertypes.ScrapedBlock) (targetMap map[string]map[string]interface{}, err error) {
	targetMap = make(map[string]map[string]interface{}) // collection addr -> token id
	events, err := util.ExtractEvents(block, "evm")
	if err != nil {
		return targetMap, err
	}

	for _, event := range events {
		for _, attr := range event.Attributes {
			if attr.Key != "log" {
				continue
			}

			var log evmtypes.Log
			if err := json.Unmarshal([]byte(attr.Value), &log); err != nil {
				return targetMap, err
			}

			if !isEvmNftLog(log) {
				continue
			}

			collectionAddr := strings.ToLower(log.Address)
			from := log.Topics[1]
			to := log.Topics[2]
			tokenId, err := convertHexStringToDecString(log.Topics[3])
			if err != nil {
				return targetMap, err
			}

			if from == emptyAddr && to != emptyAddr {
				// handle mint
				if _, ok := targetMap[collectionAddr]; !ok {
					targetMap[collectionAddr] = make(map[string]interface{})
				}
				targetMap[collectionAddr][tokenId] = nil
			} else if from != emptyAddr && to == emptyAddr {
				// handle burn
				delete(targetMap[collectionAddr], tokenId)
			}
		}
	}

	return targetMap, err
}
