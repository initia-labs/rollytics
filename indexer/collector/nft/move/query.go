package move

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	cbjson "github.com/cometbft/cometbft/libs/json"
	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/collector/nft/types"
	"github.com/initia-labs/rollytics/indexer/config"
	"golang.org/x/sync/errgroup"
)

const (
	maxRetries          = 5
	collectionStructTag = "0x1::collection::Collection"
	nftStructTag        = "0x1::nft::Nft"
)

func getCollections(colAddrs []string, client *fiber.Client, cfg *config.Config, height int64) (resourceMap map[string]string, err error) {
	return getMoveResources(colAddrs, collectionStructTag, client, cfg, height)
}

func getNfts(nftAddrs []string, client *fiber.Client, cfg *config.Config, height int64) (resourceMap map[string]string, err error) {
	return getMoveResources(nftAddrs, nftStructTag, client, cfg, height)
}

func getMoveResources(addrs []string, structTag string, client *fiber.Client, cfg *config.Config, height int64) (resourceMap map[string]string, err error) {
	resourceMap = make(map[string]string) // addr -> move resource

	if len(addrs) == 0 {
		return resourceMap, nil
	}

	var g errgroup.Group
	var mtx sync.Mutex

	for _, addr := range addrs {
		a := addr
		g.Go(func() error {
			resource, err := getMoveResource(a, structTag, client, cfg, height)
			if err != nil {
				return err
			}

			mtx.Lock()
			resourceMap[resource.Resource.Address] = resource.Resource.MoveResource
			mtx.Unlock()

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return resourceMap, err
	}

	return resourceMap, nil
}

func getMoveResource(addr string, structTag string, client *fiber.Client, cfg *config.Config, height int64) (resource QueryMoveResourceResponse, err error) {
	params := map[string]string{"struct_tag": structTag}
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := fmt.Sprintf("/initia/move/v1/accounts/%s/resources/by_struct_tag", addr)
	body, err := get(client, cfg, path, params, headers)
	if err != nil {
		return resource, err
	}

	var response QueryMoveResourceResponse
	if err := cbjson.Unmarshal(body, &response); err != nil {
		return resource, err
	}

	return response, nil
}

func get(client *fiber.Client, cfg *config.Config, path string, params map[string]string, headers map[string]string) ([]byte, error) {
	retryCount := 0
	for retryCount <= maxRetries {
		body, err := getRaw(client, cfg, path, params, headers)
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

	return nil, fmt.Errorf("failed to fetch data after %d retries", maxRetries)
}

func getRaw(client *fiber.Client, cfg *config.Config, path string, params map[string]string, headers map[string]string) (body []byte, err error) {
	baseUrl := fmt.Sprintf("%s%s", cfg.GetChainConfig().RestUrl, path)
	parsedUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	// set query params
	if params != nil {
		query := parsedUrl.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		parsedUrl.RawQuery = query.Encode()
	}

	req := client.Get(parsedUrl.String())

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
