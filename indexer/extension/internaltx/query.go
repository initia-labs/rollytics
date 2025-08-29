package internaltx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

const (
	EnableNodeVersion = "v1.1.0"
)

type NodeInfoResponse struct {
	AppVersion struct {
		Version string `json:"version"`
	} `json:"application_version"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type JSONRPCErrorResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Error   *JSONRPCError `json:"error"`
}

func CheckNodeVersion(cfg *config.Config) error {
	client := fiber.AcquireClient()
	defer fiber.ReleaseClient(client)

	path := fmt.Sprintf("/cosmos/base/tendermint/v1beta1/node_info")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetQueryTimeout())
	defer cancel()
	body, err := util.Get(ctx, cfg.GetChainConfig().RestUrl, path, nil, nil)
	if err != nil {
		return err
	}

	var response NodeInfoResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return err
	}

	// check version higher than minimum required version
	nodeVersion := strings.TrimPrefix(response.AppVersion.Version, "v")
	requiredVersion := strings.TrimPrefix(EnableNodeVersion, "v")

	nodeParts := strings.Split(nodeVersion, ".")
	requiredParts := strings.Split(requiredVersion, ".")

	// major, minor, patch
	for i := 0; i < 3 && i < len(nodeParts) && i < len(requiredParts); i++ {
		var nodeNum, reqNum int
		fmt.Sscanf(nodeParts[i], "%d", &nodeNum)
		fmt.Sscanf(requiredParts[i], "%d", &reqNum)

		if nodeNum < reqNum {
			return fmt.Errorf("node version %s is lower than required version %s", response.AppVersion.Version, EnableNodeVersion)
		} else if nodeNum > reqNum {
			return nil
		}
	}

	return nil
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetQueryTimeout()*10)
	defer cancel()
	body, err := util.Post(ctx, cfg.GetChainConfig().JsonRpcUrl, "", payload, headers)
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
