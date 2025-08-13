package internaltx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JSONRPCErrorResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Error   *JSONRPCError `json:"error"`
}

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

	body, err := util.Post(context.Background(), client, cfg.GetCoolingDuration(), cfg.GetQueryTimeout()*10, cfg.GetChainConfig().JsonRpcUrl, "", payload, headers)
	if err != nil {
		return nil, err
	}

	// fail case: check for JSON-RPC error response
	var errResp JSONRPCErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		return nil, fmt.Errorf("RPC error (code: %d): %s", errResp.Error.Code, errResp.Error.Message)
	}

	// success case: unmarshal the response into DebugCallTraceBlockResponse
	var res DebugCallTraceBlockResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &res, nil
}
