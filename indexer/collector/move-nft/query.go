package move_nft

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer/util"
)

func getMoveResource(addr string, structTag string, client *fiber.Client, cfg *config.Config, height int64) (resource QueryMoveResourceResponse, err error) {
	params := map[string]string{"struct_tag": structTag}
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := fmt.Sprintf("/initia/move/v1/accounts/%s/resources/by_struct_tag", addr)
	body, err := util.Get(client, cfg.GetCoolingDuration(), cfg.GetChainConfig().RestUrl, path, params, headers)
	if err != nil {
		return resource, err
	}

	var response QueryMoveResourceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return resource, err
	}

	return response, nil
}
