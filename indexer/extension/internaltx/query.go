package internaltx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/mod/semver"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/sentry_integration"
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
	body, err := util.Get(context.Background(), cfg.GetChainConfig().RestUrl, path, nil, nil, cfg.GetQueryTimeout())
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

func TraceCallByBlock(ctx context.Context, cfg *config.Config, client *fiber.Client, height int64) (*DebugCallTraceBlockResponse, error) {
	span, _ := sentry_integration.StartSentrySpan(ctx, "TraceCallByBlock", "Tracing internal transactions for height "+strconv.FormatInt(height, 10))
	defer span.Finish()
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
	body, err := util.Post(context.Background(), cfg.GetChainConfig().JsonRpcUrl, "", payload, headers, cfg.GetQueryTimeout()*10)
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
