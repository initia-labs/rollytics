package tx

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/indexer/config"
	"github.com/initia-labs/rollytics/indexer/util"
)

func getEvmTxs(client *fiber.Client, cfg *config.Config, height int64) (txs []EvmTx, err error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockReceipts",
		"params":  []string{fmt.Sprintf("0x%x", height)},
		"id":      1,
	}
	headers := map[string]string{"Content-Type": "application/json"}
	path := ""

	body, err := util.Post(client, cfg, path, payload, headers)
	if err != nil {
		return txs, err
	}

	var res QueryEvmTxsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return txs, err
	}

	return res.Result, nil
}
