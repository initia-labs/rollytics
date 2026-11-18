package block

import (
	"context"

	"golang.org/x/sync/singleflight"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/querier"
)

var validatorLookups singleflight.Group

func getValidator(ctx context.Context, querier *querier.Querier, validatorAddr string) (*types.ValidatorResponse, error) {
	cached, ok := cache.GetValidatorCache(validatorAddr)
	if ok {
		return cached, nil
	}
	result, err, _ := validatorLookups.Do(validatorAddr, func() (any, error) {
		if cached, ok := cache.GetValidatorCache(validatorAddr); ok {
			return cached, nil
		}
		validator, err := querier.GetValidator(ctx, validatorAddr)
		if err != nil {
			return nil, err
		}
		cache.SetValidatorCache(validatorAddr, validator)
		return validator, nil
	})
	if err != nil {
		return nil, err
	}

	return result.(*types.ValidatorResponse), nil
}
