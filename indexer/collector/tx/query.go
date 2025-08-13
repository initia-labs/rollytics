package tx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func getCosmosTxs(client *fiber.Client, cfg *config.Config, height int64, txCount int) (txs []RestTx, err error) {
	params := map[string]string{"pagination.limit": "1000"}
	path := fmt.Sprintf("/cosmos/tx/v1beta1/txs/block/%d", height)

	body, err := util.Get(context.Background(), client, cfg.GetCoolingDuration(), cfg.GetQueryTimeout(), cfg.GetChainConfig().RestUrl, path, params, nil)
	if err != nil {
		return txs, err
	}

	var response QueryRestTxsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return txs, err
	}

	if len(response.Txs) != txCount {
		return txs, fmt.Errorf("expected %d txs but got %d for height %d", txCount, len(response.Txs), height)
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

	body, err := util.Post(context.Background(), client, cfg.GetCoolingDuration(), cfg.GetQueryTimeout(), cfg.GetChainConfig().JsonRpcUrl, path, payload, headers)
	if err != nil {
		return txs, err
	}

	var res QueryEvmTxsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return txs, err
	}

	return res.Result, nil
}
