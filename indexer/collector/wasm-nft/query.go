package wasm_nft

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/util"
	"golang.org/x/sync/errgroup"
)

var (
	qreqContractInfo   = []byte("{\"contract_info\":{}}")
	qreqCollectionInfo = []byte("{\"collection_info\":{}}")
)

func getCollectionInfo(collectionAddr string, client *fiber.Client, cfg *config.Config, height int64) (info CacheCollectionInfo, err error) {
	var g errgroup.Group
	getCollectionNameRes := make(chan string, 1)
	getCollectionCreatorRes := make(chan string, 1)

	g.Go(func() error {
		defer close(getCollectionNameRes)
		name, err := getCollectionName(collectionAddr, client, cfg, height)
		if err != nil {
			return err
		}
		getCollectionNameRes <- name
		return nil
	})

	g.Go(func() error {
		defer close(getCollectionCreatorRes)
		creator, err := getCollectionCreator(collectionAddr, client, cfg, height)
		if err != nil {
			return err
		}
		getCollectionCreatorRes <- creator
		return nil
	})

	if err := g.Wait(); err != nil {
		return info, err
	}

	name := <-getCollectionNameRes
	creator := <-getCollectionCreatorRes

	return CacheCollectionInfo{
		Name:    name,
		Creator: creator,
	}, nil
}

func getCollectionName(collectionAddr string, client *fiber.Client, cfg *config.Config, height int64) (name string, err error) {
	queryData := base64.StdEncoding.EncodeToString(qreqContractInfo)
	body, err := querySmart(collectionAddr, queryData, client, cfg, height)
	if err != nil {
		return name, err
	}

	var response QueryContractInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return name, err
	}

	return response.Data.Name, nil
}

func getCollectionCreator(collectionAddr string, client *fiber.Client, cfg *config.Config, height int64) (creator string, err error) {
	queryData := base64.StdEncoding.EncodeToString(qreqCollectionInfo)
	body, err := querySmart(collectionAddr, queryData, client, cfg, height)
	if err != nil {
		return creator, err
	}

	var response QueryCollectionInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return creator, err
	}

	return response.Data.Creator, nil
}

func querySmart(contractAddr, queryData string, client *fiber.Client, cfg *config.Config, height int64) (response []byte, err error) {
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := fmt.Sprintf("/cosmwasm/wasm/v1/contract/%s/smart/%s", contractAddr, queryData)
	return util.Get(client, cfg, path, nil, headers)
}
