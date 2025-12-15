package querier

import (
	"context"
	"fmt"
	"time"

	"github.com/initia-labs/rollytics/types"
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
	res, err := executeWithEndpointRotation[types.QueryMoveResourceResponse](ctx, q.RestUrls, fetchMoveResource(addr, structTag, height, queryTimeout))
	if err != nil {
		return nil, err
	}
	return res, nil
}
