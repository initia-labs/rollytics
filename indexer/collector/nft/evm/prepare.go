package evm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/gofiber/fiber/v2"
	evmtypes "github.com/initia-labs/minievm/x/evm/types"
	"github.com/initia-labs/rollytics/indexer/collector/nft/types"
	"github.com/initia-labs/rollytics/indexer/config"
	indexertypes "github.com/initia-labs/rollytics/indexer/types"
	"golang.org/x/sync/errgroup"
)

const (
	nftTopic  = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	emptyAddr = "0000000000000000000000000000000000000000"
)

func Prepare(block indexertypes.ScrappedBlock, cfg *config.Config) (data types.CacheData, err error) {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	targetMap, err := filterEvmData(block)
	if err != nil {
		return data, err
	}

	var collectionAddrs []string
	var queryData []QueryTokenUriData
	for collectionAddr, tokenIdMap := range targetMap {
		collectionAddrs = append(collectionAddrs, collectionAddr)
		for tokenId := range tokenIdMap {
			queryData = append(queryData, QueryTokenUriData{
				CollectionAddr: collectionAddr,
				TokenId:        tokenId,
			})
		}
	}

	var g errgroup.Group
	getCollectionNamesRes := make(chan map[string]string, 1)
	getTokenUrisRes := make(chan map[string]string, 1)

	g.Go(func() error {
		defer close(getCollectionNamesRes)
		nameMap, err := getCollectionNames(collectionAddrs, client, cfg, block.Height)
		if err != nil {
			return err
		}
		getCollectionNamesRes <- nameMap
		return nil
	})

	g.Go(func() error {
		defer close(getTokenUrisRes)
		uriMap, err := getTokenUris(queryData, client, cfg, block.Height)
		if err != nil {
			return err
		}
		getTokenUrisRes <- uriMap
		return nil
	})

	nameMap := <-getCollectionNamesRes
	uriMap := <-getTokenUrisRes

	return types.CacheData{
		CollectionMap: nameMap,
		NftMap:        uriMap,
	}, nil
}

func filterEvmData(block indexertypes.ScrappedBlock) (targetMap map[string]map[string]interface{}, err error) {
	targetMap = make(map[string]map[string]interface{}) // collection addr -> token id

	events, err := getEvents(block)
	if err != nil {
		return targetMap, err
	}

	for _, event := range events {
		if event.Type != "evm" {
			continue
		}

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

			collectionAddr := log.Address
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

	return
}

func getEvents(block indexertypes.ScrappedBlock) (events []EventWithHash, err error) {
	for _, event := range block.BeginBlock {
		events = append(events, EventWithHash{
			TxHash: "",
			Event:  event,
		})
	}

	for txIndex, txRaw := range block.Txs {
		txByte, err := base64.StdEncoding.DecodeString(txRaw)
		if err != nil {
			return events, err
		}
		txHash := fmt.Sprintf("%X", tmhash.Sum(txByte))
		txRes := block.TxResults[txIndex]
		for _, event := range txRes.Events {
			events = append(events, EventWithHash{
				TxHash: txHash,
				Event:  event,
			})
		}
	}

	for _, event := range block.EndBlock {
		events = append(events, EventWithHash{
			TxHash: "",
			Event:  event,
		})
	}

	return events, nil
}

func isEvmNftLog(log evmtypes.Log) bool {
	return len(log.Topics) == 4 && log.Topics[0] == nftTopic && log.Data == "0x"
}

func convertHexStringToDecString(hex string) (string, error) {
	hex = strings.TrimPrefix(hex, "0x")
	bi, ok := new(big.Int).SetString(hex, 16)
	if !ok {
		return "", errors.New("failed to convert hex to dec")
	}
	return bi.String(), nil
}
