package block

import (
	"context"

	"golang.org/x/sync/singleflight"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/querier"
)

var validatorLookups singleflight.Group

func getValidator(ctx context.Context, querier *querier.Querier, validatorAddr string) (*types.ValidatorResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cached, ok := cache.GetValidatorCache(validatorAddr)
	if ok {
		return cached, nil
	}
	result := validatorLookups.DoChan(validatorAddr, func() (any, error) {
		if cached, ok := cache.GetValidatorCache(validatorAddr); ok {
			return cached, nil
		}
		// The shared refresh must outlive individual callers, but remain bounded.
		refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.DefaultQueryTimeout)
		defer cancel()
		validator, err := querier.GetValidator(refreshCtx, validatorAddr)
		if err != nil {
			return nil, err
		}
		cache.SetValidatorCache(validatorAddr, validator)
		return validator, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-result:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Val.(*types.ValidatorResponse), nil
	}
}
