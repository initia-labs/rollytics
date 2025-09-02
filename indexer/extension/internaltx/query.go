package internaltx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/mod/semver"

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

	path := "/cosmos/base/tendermint/v1beta1/node_info"
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
	nodeVersion := response.AppVersion.Version
	if !semver.IsValid(nodeVersion) {
		nodeVersion = "v" + nodeVersion
	}

	if semver.Compare(nodeVersion, EnableNodeVersion) < 0 {
		return fmt.Errorf("node version %s is lower than required version %s", response.AppVersion.Version, EnableNodeVersion)
	}

	return nil
}

func TraceCallByBlock(cfg *config.Config, client *fiber.Client, height int64) (*DebugCallTraceBlockResponse, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "debug_traceBlockByNumber",
		"params": []any{
			fmt.Sprintf("0x%x", height),
			map[string]any{
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
