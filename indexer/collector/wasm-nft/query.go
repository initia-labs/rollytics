package wasm_nft

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

var qreqContractInfo = []byte("{\"contract_info\":{}}")

func getCollectionName(collectionAddr string, client *fiber.Client, cfg *config.Config, height int64) (name string, err error) {
	queryData := base64.StdEncoding.EncodeToString(qreqContractInfo)
	body, err := querySmart(collectionAddr, queryData, client, cfg, height)
	if err != nil {
		return name, err
	}

	var response QueryContractInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return name, err
	}

	return response.Data.Name, nil
}

func querySmart(contractAddr, queryData string, client *fiber.Client, cfg *config.Config, height int64) (response []byte, err error) {
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := fmt.Sprintf("/cosmwasm/wasm/v1/contract/%s/smart/%s", contractAddr, queryData)
	return util.Get(context.Background(), client, cfg.GetCoolingDuration(), cfg.GetQueryTimeout(), cfg.GetChainConfig().RestUrl, path, nil, headers)
}
