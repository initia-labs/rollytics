package block

import (
	"context"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/querier"
)

func getValidator(ctx context.Context, querier *querier.Querier, validatorAddr string) (*types.Validator, error) {
	cached, ok := cache.GetValidatorCache(validatorAddr)
	if ok {
		return cached, nil
	}
	validator, err := querier.GetValidator(ctx, validatorAddr)
	if err != nil {
		return nil, err
	}

	cache.SetValidatorCache(validator)

	return validator, nil
}
