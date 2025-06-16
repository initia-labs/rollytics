package tx

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/util"
	"github.com/initia-labs/rollytics/types"
)

func getRestTxs(client *fiber.Client, cfg *config.Config, height int64) (txs []RestTx, err error) {
	path := fmt.Sprintf("/cosmos/tx/v1beta1/txs/block/%d", height)
	body, err := util.Get(client, cfg.GetCoolingDuration(), cfg.GetChainConfig().RestUrl, path, nil, nil)
	if err != nil {
		return txs, err
	}

	var response QueryRestTxsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return txs, err
	}

	return response.Txs, nil
}

func getEvmTxs(client *fiber.Client, cfg *config.Config, height int64) (txs []types.EvmTx, err error) {
	if cfg.GetVmType() != types.EVM {
		return
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockReceipts",
		"params":  []string{fmt.Sprintf("0x%x", height)},
		"id":      1,
	}
	headers := map[string]string{"Content-Type": "application/json"}
	path := ""

	body, err := util.Post(client, cfg.GetCoolingDuration(), cfg.GetChainConfig().JsonRpcUrl, path, payload, headers)
	if err != nil {
		return txs, err
	}

	var res QueryEvmTxsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return txs, err
	}

	return res.Result, nil
}
