package internaltx

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

func TraceCallByBlock(cfg *config.Config, client *fiber.Client, height int64) (*DebugCallTraceBlockResponse, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "debug_traceBlockByNumber",
		"params": []interface{}{
			fmt.Sprintf("0x%x", height),
			map[string]interface{}{
				"tracer": "callTracer",
			},
		},
		"id": 1,
	}
	headers := map[string]string{"Content-Type": "application/json"}

	body, err := util.Post(client, cfg.GetCoolingDuration(), cfg.GetQueryTimeout()*10, cfg.GetChainConfig().JsonRpcUrl, "", payload, headers)
	if err != nil {
		return nil, err
	}

	var res DebugCallTraceBlockResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
