package evm

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/gofiber/fiber/v2"
	movetypes "github.com/initia-labs/initia/x/move/types"
	"github.com/initia-labs/minievm/x/evm/contracts/erc721"
	vmtypes "github.com/initia-labs/movevm/types"
	"github.com/initia-labs/rollytics/indexer/collector/nft/types"
	"github.com/initia-labs/rollytics/indexer/config"
	"golang.org/x/sync/errgroup"
)

const (
	maxRetries = 5
)

var (
	stdAddr = movetypes.ConvertVMAddressToSDKAddress(vmtypes.StdAddress).String()
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
	payload := map[string]string{
		"sender":       sender,
		"contract_add": contractAddr,
		"input":        fmt.Sprintf("0x%s", hex.EncodeToString(input)),
		"value":        "0",
	}
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := "/minievm/evm/v1/call"
	body, err := post(client, cfg, path, payload, headers)
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

func post(client *fiber.Client, cfg *config.Config, path string, payload map[string]string, headers map[string]string) ([]byte, error) {
	retryCount := 0
	for retryCount <= maxRetries {
		body, err := postRaw(client, cfg, path, payload, headers)
		if err == nil {
			return body, nil
		}

		// handle case of querying future height
		if strings.HasPrefix(fmt.Sprintf("%+v", err), "invalid height") {
			time.Sleep(cfg.GetCoolingDuration())
			continue
		}

		retryCount++
		if retryCount > maxRetries {
			return nil, err
		}
		time.Sleep(cfg.GetCoolingDuration())
	}

	return nil, fmt.Errorf("failed to post data after %d retries", maxRetries)
}

func postRaw(client *fiber.Client, cfg *config.Config, path string, payload map[string]string, headers map[string]string) (body []byte, err error) {
	baseUrl := fmt.Sprintf("%s%s", cfg.GetChainConfig().RestUrl, path)
	req := client.Post(baseUrl)

	// set payload
	if payload != nil {
		req = req.JSON(payload)
	}

	// set header
	for key, value := range headers {
		req.Set(key, value)
	}

	code, body, errs := req.Timeout(5 * time.Second).Bytes()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	if code != fiber.StatusOK {
		if code == fiber.StatusInternalServerError {
			var res types.ErrorResponse
			if err := json.Unmarshal(body, &res); err != nil {
				return body, err
			}

			if res.Message == "codespace sdk code 26: invalid height: cannot query with height in the future; please provide a valid height" {
				return nil, fmt.Errorf("invalid height")
			}
		}

		return nil, fmt.Errorf("http response: %d, body: %s", code, string(body))
	}

	return body, nil
}
