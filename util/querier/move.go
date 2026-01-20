package querier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
)

const (
	moveResourcePath = "/initia/move/v1/accounts/%s/resources/by_struct_tag"
)

func fetchMoveResource(addr string, structTag string, height int64, timeout time.Duration) func(ctx context.Context, endpointURL string) (*types.QueryMoveResourceResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.QueryMoveResourceResponse, error) {
		params := map[string]string{"struct_tag": structTag}
		headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
		path := fmt.Sprintf(moveResourcePath, addr)
		body, err := Get(ctx, endpointURL, path, params, headers, timeout)
		if err != nil {
			return nil, err
		}
		resource, err := extractResponse[types.QueryMoveResourceResponse](body)
		if err != nil {
			return nil, err
		}
		return &resource, nil
	}
}

func (q *Querier) GetMoveResource(ctx context.Context, addr string, structTag string, height int64) (resource *types.QueryMoveResourceResponse, err error) {
	res, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchMoveResource(addr, structTag, height, queryTimeout))
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (q *Querier) GetMoveDenomByMetadataAddr(ctx context.Context, metadataAddr string) (denom string, err error) {
	// Check cache first
	key := strings.ToLower(metadataAddr)
	if denom, ok := cache.GetMoveDenomCache(key); ok {
		return denom, nil
	}

	resourceResponse, err := q.GetMoveResource(ctx, metadataAddr, types.MoveMetadataTypeTag, 0)
	if err != nil {
		return "", err
	}

	var resource types.MoveResource
	err = json.Unmarshal([]byte(resourceResponse.Resource.MoveResource), &resource)
	if err != nil {
		return "", err
	}

	var metadata types.MoveFungibleAssetMetadata
	err = json.Unmarshal(resource.Data, &metadata)
	if err != nil {
		return "", err
	}

	if metadata.Decimals == 0 && metadata.Symbol != "" {
		denom = metadata.Symbol
	} else {
		denom = strings.Replace(metadataAddr, "0x", "move/", 1)
	}

	// Cache the result
	cache.SetMoveDenomCache(key, denom)

	return denom, nil
}
