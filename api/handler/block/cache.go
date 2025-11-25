package block

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/initia-labs/rollytics/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

type ValidatorResponse struct {
	Validator Validator `json:"validator"`
}

type Validator struct {
	Moniker         string          `json:"moniker"`
	OperatorAddress string          `json:"operator_address"`
	ConsensusPubkey ConsensusPubkey `json:"consensus_pubkey"`
}

type ConsensusPubkey struct {
	Type string `json:"@type"`
	Key  string `json:"key"`
}

// cache for validators
var (
	validatorCacheOnce sync.Once
	validatorCache     *cache.Cache[string, *Validator]
)

func initValidatorCache(cfg *config.Config) {
	validatorCacheOnce.Do(func() {
		cacheSize := cfg.GetCacheSize()
		validatorCache = cache.New[string, *Validator](cacheSize)
	})
}

func getValidator(validatorAddr string, cfg *config.Config) (*Validator, error) {
	cached, ok := validatorCache.Get(validatorAddr)
	if ok {
		return cached, nil
	}
	path := fmt.Sprintf("/opinit/opchild/v1/validator/%s", validatorAddr)
	body, err := util.Get(context.Background(), cfg.GetChainConfig().RestUrl, path, nil, nil, cfg.GetQueryTimeout())
	if err != nil {
		return nil, err
	}

	var response ValidatorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	validatorCache.Set(validatorAddr, &response.Validator)

	return &response.Validator, nil
}
