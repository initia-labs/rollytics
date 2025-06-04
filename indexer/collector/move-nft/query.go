package move_nft

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/util"
	"golang.org/x/sync/errgroup"
)

const (
	collectionStructTag = "0x1::collection::Collection"
	nftStructTag        = "0x1::nft::Nft"
)

func getCollectionResources(colAddrs []string, client *fiber.Client, cfg *config.Config, height int64) (resourceMap map[string]string, err error) {
	return getMoveResources(colAddrs, collectionStructTag, client, cfg, height)
}

func getNftResources(nftAddrs []string, client *fiber.Client, cfg *config.Config, height int64) (resourceMap map[string]string, err error) {
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
	body, err := util.Get(client, cfg, path, params, headers)
	if err != nil {
		return resource, err
	}

	var response QueryMoveResourceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return resource, err
	}

	return response, nil
}
