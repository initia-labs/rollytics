package util

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// BlockResponse represents the response from /cosmos/base/tendermint/v1beta1/blocks/latest
type BlockResponse struct {
	Block struct {
		Header struct {
			Height string `json:"height"`
		} `json:"header"`
	} `json:"block"`
}

func GetLatestHeight(client *fiber.Client, cfg *config.Config) (int64, error) {
	path := "/cosmos/base/tendermint/v1beta1/blocks/latest"

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetQueryTimeout())
	defer cancel()
	body, err := util.Get(ctx, cfg.GetChainConfig().RestUrl, path, nil, nil)
	if err != nil {
		return 0, err
	}

	var response BlockResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, err
	}

	height := int64(0)
	if _, err := fmt.Sscanf(response.Block.Header.Height, "%d", &height); err != nil {
		return 0, types.NewInvalidValueError("height", response.Block.Header.Height, "failed to parse as integer")
	}

	return height, nil
}
