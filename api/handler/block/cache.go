package block

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

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
	ConsPower       string          `json:"cons_power"`
}

type ConsensusPubkey struct {
	Type string `json:"@type"`
	Key  string `json:"key"`
}

// cache for validators
var validatorCache = cache.New[string, *Validator](100)

func getValidator(validatorAddr string, cfg *config.Config) (*Validator, error) {
	cached, ok := validatorCache.Get(validatorAddr)
	if ok {
		return cached, nil
	}

	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	path := fmt.Sprintf("/opinit/opchild/v1/validator/%s", validatorAddr)
	body, err := util.Get(client, cfg.GetCoolingDuration(), cfg.GetQueryTimeout(), cfg.GetChainConfig().RestUrl, path, nil, nil)
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
