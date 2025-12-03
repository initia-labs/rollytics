package querier

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/initia-labs/minievm/x/evm/contracts/erc721"

	"github.com/initia-labs/rollytics/sentry_integration"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/cache"
)

const (
	evmCallPath            = "/minievm/evm/v1/call"
	evmContractByDenomPath = "/minievm/evm/v1/contracts/by_denom"
)

func (q *Querier) GetCollectionName(ctx context.Context, collectionAddr string, height int64) (name string, err error) {
	abi, err := erc721.Erc721MetaData.GetAbi()
	if err != nil {
		return name, err
	}

	input, err := abi.Pack("name")
	if err != nil {
		return name, err
	}

	callRes, err := q.evmCall(ctx, collectionAddr, input, height)
	if err != nil {
		return name, err
	}

	err = abi.UnpackIntoInterface(&name, "name", callRes)
	return
}

func (q *Querier) GetTokenUri(ctx context.Context, collectionAddr, tokenIdStr string, height int64) (tokenUri string, err error) {
	abi, err := erc721.Erc721MetaData.GetAbi()
	if err != nil {
		return tokenUri, err
	}

	tokenId, ok := new(big.Int).SetString(tokenIdStr, 10)
	if !ok {
		return tokenUri, types.NewInvalidValueError("token_id", tokenIdStr, "must be a valid decimal number")
	}
	input, err := abi.Pack("tokenURI", tokenId)
	if err != nil {
		return tokenUri, err
	}

	callRes, err := q.evmCall(ctx, collectionAddr, input, height)
	if err != nil {
		return tokenUri, err
	}

	err = abi.UnpackIntoInterface(&tokenUri, "tokenURI", callRes)
	return
}

func fetchEvmCall(contractAddr string, input []byte, height int64, timeout time.Duration) func(ctx context.Context, endpointURL string) (*QueryCallResponse, error) {
	return func(ctx context.Context, endpointURL string) (*QueryCallResponse, error) {
		payload := map[string]any{
			"sender":        "init1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqpqr5e3d",
			"contract_addr": contractAddr,
			"input":         fmt.Sprintf("0x%s", hex.EncodeToString(input)),
			"value":         "0",
		}
		headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
		body, err := Post(ctx, endpointURL, evmCallPath, payload, headers, timeout)
		if err != nil {
			return nil, err
		}
		callRes, err := extractResponse[QueryCallResponse](body)
		if err != nil {
			return nil, err
		}

		if callRes.Error != "" {
			return nil, fmt.Errorf("error from evm call: %s", callRes.Error)
		}

		return &callRes, nil
	}
}

func (q *Querier) evmCall(ctx context.Context, contractAddr string, input []byte, height int64) (response []byte, err error) {
	callRes, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchEvmCall(contractAddr, input, height, queryTimeout))
	if err != nil {
		return response, err
	}
	return hex.DecodeString(strings.TrimPrefix(callRes.Response, "0x"))
}

func fetchEvmTxs(height int64, timeout time.Duration) func(ctx context.Context, endpointURL string) (*types.QueryEvmTxsResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.QueryEvmTxsResponse, error) {
		payload := map[string]any{
			"jsonrpc": "2.0",
			"method":  "eth_getBlockReceipts",
			"params":  []string{fmt.Sprintf("0x%x", height)},
			"id":      1,
		}
		headers := map[string]string{"Content-Type": "application/json"}
		body, err := Post(ctx, endpointURL, "", payload, headers, timeout)
		if err != nil {
			return nil, err
		}
		txs, err := extractResponse[types.QueryEvmTxsResponse](body)
		if err != nil {
			return nil, err
		}
		return &txs, nil
	}
}

func (q *Querier) GetEvmTxs(ctx context.Context, height int64) (txs []types.EvmTx, err error) {
	if q.VmType != types.EVM {
		return
	}

	res, err := executeWithEndpointRotation(ctx, q.JsonRpcUrls, fetchEvmTxs(height, queryTimeout))
	if err != nil {
		return txs, err
	}

	return res.Result, nil
}

func fetchEvmContractByDenom(denom string) func(ctx context.Context, endpointURL string) (*types.EvmContractByDenomResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.EvmContractByDenomResponse, error) {
		body, err := Get(ctx, endpointURL, evmContractByDenomPath, map[string]string{"denom": denom}, nil, queryTimeout)
		if err != nil {
			return nil, err
		}
		response, err := extractResponse[types.EvmContractByDenomResponse](body)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}
}

// GetEvmContractByDenom queries the MiniEVM API for a contract address by denom
// and caches the result. It returns the contract address or an error.
func (q *Querier) GetEvmContractByDenom(ctx context.Context, denom string) (string, error) {
	if strings.HasPrefix(denom, "0x") {
		return denom, nil
	}

	// Check cache first
	if address, ok := cache.GetEvmDenomContractCache(denom); ok {
		return address, nil
	}

	// ibc/UPPERCASE
	// l2/lowercase
	// evm/AnyCase
	if strings.HasPrefix(denom, "ibc/") {
		denom = fmt.Sprintf("ibc/%s", strings.ToUpper(denom[4:]))
	} else if strings.ToLower(denom) == "gas" {
		denom = "GAS"
	}

	response, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchEvmContractByDenom(denom))
	if err != nil {
		return "", err
	}

	// Validate the response
	if response == nil || response.Address == "" {
		return "", fmt.Errorf("empty contract address returned for denom %s", denom)
	}

	// Cache the result
	address := strings.ToLower(response.Address)
	cache.SetEvmDenomContractCache(denom, address)

	return address, nil
}

func fetchTraceCallByBlock(height int64, timeout time.Duration) func(ctx context.Context, endpointURL string) (*types.DebugCallTraceBlockResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.DebugCallTraceBlockResponse, error) {
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
		body, err := Post(ctx, endpointURL, "", payload, headers, timeout)
		if err != nil {
			return nil, err
		}

		errResp, err := extractResponse[types.JSONRPCErrorResponse](body)
		if err == nil && errResp.Error != nil {
			return nil, fmt.Errorf("RPC error (code: %d): %s", errResp.Error.Code, errResp.Error.Message)
		}

		// success case: unmarshal the response into DebugCallTraceBlockResponse
		response, err := extractResponse[types.DebugCallTraceBlockResponse](body)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}
}

func (q *Querier) TraceCallByBlock(ctx context.Context, height int64) (*types.DebugCallTraceBlockResponse, error) {
	res, err := executeWithEndpointRotation(ctx, q.JsonRpcUrls, fetchTraceCallByBlock(height, queryTimeout))
	if err != nil {
		return nil, err
	}
	return res, nil
}

// parseLatestHeightFromBatch extracts and parses the latest block height from batch responses
func parseLatestHeightFromBatch(batchResponses []types.JSONRPCResponse) (int64, error) {
	for _, resp := range batchResponses {
		if resp.ID == 0 {
			// Check for JSON-RPC error
			if resp.Error != nil {
				return 0, fmt.Errorf("JSON-RPC error for eth_blockNumber: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
			}

			// Remove 0x prefix and parse hex
			heightValue, err := strconv.ParseInt(strings.TrimPrefix(resp.Result, "0x"), 16, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse eth_blockNumber result: %w", err)
			}
			return heightValue, nil
		}
	}
	return 0, fmt.Errorf("eth_blockNumber response (ID 0) not found in batch")
}

func fetchERC20Balances(erc20Address string, addresses []string, height int64) func(ctx context.Context, endpointURL string) (*[]types.JSONRPCResponse, error) {
	return func(ctx context.Context, endpointURL string) (*[]types.JSONRPCResponse, error) {
		// queryBatchBalances queries balances for a batch of addresses at a specific height
		// balanceOf function selector: keccak256("balanceOf(address)")[:4] = 0x70a08231
		const balanceOfSelector = "0x70a08231"

		batchRequests := make([]types.JSONRPCRequest, 0, len(addresses)+1)
		batchRequests = append(batchRequests, types.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "eth_blockNumber",
			Params:  []any{},
			ID:      0,
		})

		// Create batch of JSON-RPC requests
		for idx, address := range addresses {
			// Prepare the call data: balanceOf(address)
			// Format: 0x70a08231 + 000000000000000000000000 + address (without 0x)
			addressParam := strings.TrimPrefix(address, "0x")

			// Pad address to 32 bytes (64 hex chars) - efficient single allocation
			if len(addressParam) < 64 {
				addressParam = strings.Repeat("0", 64-len(addressParam)) + addressParam
			}

			callData := balanceOfSelector + addressParam

			// Convert height to hex format (0x prefix)
			blockParam := fmt.Sprintf("0x%x", height)

			// Create JSON-RPC request with unique ID
			rpcReq := types.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "eth_call",
				Params: []any{
					map[string]string{
						"to":   erc20Address,
						"data": callData,
					},
					blockParam,
				},
				ID: idx + 1, // Unique ID for each request in the batch
			}

			batchRequests = append(batchRequests, rpcReq)
		}

		// Send JSON-RPC batch request using util.Post
		headers := map[string]string{
			"Content-Type": "application/json",
		}

		var batchResponses []types.JSONRPCResponse

		// TODO: handle more appropriately
		respBody, err := Post(ctx, endpointURL, "", batchRequests, headers, queryTimeout)
		if err != nil {
			return nil, err
		}

		batchResponses, err = extractResponse[[]types.JSONRPCResponse](respBody)
		if err != nil {
			return nil, err
		}

		// Process each response in the batch
		if len(batchResponses) != len(addresses)+1 {
			return nil, fmt.Errorf("batch response count mismatch: expected %d, got %d", len(addresses)+1, len(batchResponses))
		}

		// Parse the latest height from the batch response
		latestHeight, err := parseLatestHeightFromBatch(batchResponses)
		if err != nil {
			return nil, err
		}

		// Check if the latest height is less than the requested height
		if latestHeight < height {
			return nil, fmt.Errorf("latest height is less than requested height: %d < %d", latestHeight, height)
		}

		return &batchResponses, nil
	}
}

func (q *Querier) QueryERC20Balances(ctx context.Context, erc20Address string, addresses []string, height int64) ([]types.JSONRPCResponse, error) {
	res, err := executeWithEndpointRotation(ctx, q.JsonRpcUrls, fetchERC20Balances(erc20Address, addresses, height))
	if err != nil {
		return nil, err
	}
	return *res, nil
}
