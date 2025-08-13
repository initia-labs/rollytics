package move_nft

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

func getMoveResource(addr string, structTag string, client *fiber.Client, cfg *config.Config, height int64) (resource QueryMoveResourceResponse, err error) {
	params := map[string]string{"struct_tag": structTag}
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := fmt.Sprintf("/initia/move/v1/accounts/%s/resources/by_struct_tag", addr)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetQueryTimeout())
	defer cancel()
	body, err := util.Get(ctx, cfg.GetChainConfig().RestUrl, path, params, headers)
	if err != nil {
		return resource, err
	}

	var response QueryMoveResourceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return resource, err
	}

	return response, nil
}
