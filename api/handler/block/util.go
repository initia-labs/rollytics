package block

import (
	"fmt"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
)

type ValidatorResponse struct {
	Moniker         string          `json:"moniker"`
	OperatorAddress string          `json:"operator_address"`
	ConsensusPubkey ConsensusPubkey `json:"consensus_pubkey"`
	ConsPower       string          `json:"cons_power"`
}

type ConsensusPubkey struct {
	Type string `json:"@type"`
	Key  string `json:"key"`
}

// cache for validator responses
var (
	validatorCache     = make(map[string]*ValidatorResponse)
	validatorCacheLock sync.RWMutex
)

func getValidator(restUrl string, validatorAddr string) (*ValidatorResponse, error) {
	validatorCacheLock.RLock()
	cached, ok := validatorCache[validatorAddr]
	validatorCacheLock.RUnlock()
	if ok {
		return cached, nil
	}

	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	req := client.Get(fmt.Sprintf("%s%s", restUrl, fmt.Sprintf("/opinit/opchild/v1/validator/%s", validatorAddr)))

	code, body, errs := req.Timeout(5 * time.Second).Bytes()

	res := &ValidatorResponse{}
	err := json.Unmarshal(body, res)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal validator response: %w", err)
	}
	if code != fiber.StatusOK {
		return nil, fmt.Errorf("failed to fetch validator info: %s", errs)
	}

	validatorCacheLock.Lock()
	validatorCache[validatorAddr] = res
	validatorCacheLock.Unlock()

	return res, nil
}
