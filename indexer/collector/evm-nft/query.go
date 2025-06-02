package evm_nft

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/minievm/x/evm/contracts/erc721"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/util"
	"golang.org/x/sync/errgroup"
)

const (
	maxRetries = 5
	stdAddr    = "init1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqpqr5e3d"
)

func getCollectionNames(collectionAddrs []string, client *fiber.Client, cfg *config.Config, height int64) (nameMap map[string]string, err error) {
	nameMap = make(map[string]string)

	if len(collectionAddrs) == 0 {
		return nameMap, nil
	}

	var g errgroup.Group
	var mtx sync.Mutex

	for _, collectionAddr := range collectionAddrs {
		addr := collectionAddr
		g.Go(func() error {
			name, err := getCollectionName(addr, client, cfg, height)
			if err != nil {
				return err
			}

			mtx.Lock()
			nameMap[addr] = name
			mtx.Unlock()

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return nameMap, err
	}

	return nameMap, nil
}

func getCollectionName(collectionAddr string, client *fiber.Client, cfg *config.Config, height int64) (name string, err error) {
	abi, err := erc721.Erc721MetaData.GetAbi()
	if err != nil {
		return name, err
	}

	input, err := abi.Pack("name")
	if err != nil {
		return name, err
	}

	callRes, err := evmCall(stdAddr, collectionAddr, input, client, cfg, height)
	if err != nil {
		return name, err
	}

	err = abi.UnpackIntoInterface(&name, "name", callRes)
	return
}

func getTokenUris(queryData []QueryTokenUriData, client *fiber.Client, cfg *config.Config, height int64) (uriMap map[string]string, err error) {
	uriMap = make(map[string]string)

	if len(queryData) == 0 {
		return uriMap, nil
	}

	var g errgroup.Group
	var mtx sync.Mutex

	for _, data := range queryData {
		d := data
		g.Go(func() error {
			tokenUri, err := getTokenUri(d.CollectionAddr, d.TokenId, client, cfg, height)
			if err != nil {
				return err
			}

			mtx.Lock()
			uriMap[fmt.Sprintf("%s%s", d.CollectionAddr, d.TokenId)] = tokenUri
			mtx.Unlock()

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return uriMap, err
	}

	return uriMap, nil
}

func getTokenUri(collectionAddr, tokenId string, client *fiber.Client, cfg *config.Config, height int64) (tokenUri string, err error) {
	abi, err := erc721.Erc721MetaData.GetAbi()
	if err != nil {
		return tokenUri, err
	}

	input, err := abi.Pack("tokenURI", tokenId)
	if err != nil {
		return tokenUri, err
	}

	callRes, err := evmCall(stdAddr, collectionAddr, input, client, cfg, height)
	if err != nil {
		return tokenUri, err
	}

	err = abi.UnpackIntoInterface(&tokenUri, "tokenURI", callRes)
	return
}

func evmCall(sender, contractAddr string, input []byte, client *fiber.Client, cfg *config.Config, height int64) (response []byte, err error) {
	payload := map[string]interface{}{
		"sender":       sender,
		"contract_add": contractAddr,
		"input":        fmt.Sprintf("0x%s", hex.EncodeToString(input)),
		"value":        "0",
	}
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := "/minievm/evm/v1/call"
	body, err := util.Post(client, cfg, path, payload, headers)
	if err != nil {
		return response, err
	}

	var callRes QueryCallResponse
	if err := cbjson.Unmarshal(body, &callRes); err != nil {
		return response, err
	}

	if callRes.Error != "" {
		return response, fmt.Errorf("error from evm call: %s", callRes.Error)
	}

	return hex.DecodeString(strings.TrimPrefix(callRes.Response, "0x"))
}
