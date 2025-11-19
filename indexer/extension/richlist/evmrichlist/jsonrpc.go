package evmrichlist

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"

	sdkmath "cosmossdk.io/math"
	"golang.org/x/sync/errgroup"

	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/util"
)

// queryERC20Balances queries the balances of multiple addresses for a specific ERC20 token via JSON-RPC.
// It returns a map of AddressWithID to balance (as sdkmath.Int).
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - jsonrpcURL: The JSON-RPC endpoint URL
//   - erc20Address: The ERC20 token contract address (with 0x prefix)
//   - addresses: List of addresses with account IDs to query
//   - height: The block height to query at
//
// The function uses the eth_call method to call the balanceOf function on the ERC20 contract.
// The balanceOf function signature is: balanceOf(address) returns (uint256)
// Function selector: 0x70a08231
func queryERC20Balances(ctx context.Context, jsonrpcURL string, erc20Address string, addresses []richlistutils.AddressWithID, height int64) (map[richlistutils.AddressWithID]sdkmath.Int, error) {
	if len(addresses) == 0 {
		return make(map[richlistutils.AddressWithID]sdkmath.Int), nil
	}

	balances := make(map[richlistutils.AddressWithID]sdkmath.Int, len(addresses))

	const batchSize = 1000
	const maxConcurrent = 10

	// Create batches
	var batches [][]richlistutils.AddressWithID
	for i := 0; i < len(addresses); i += batchSize {
		end := min(i+batchSize, len(addresses))
		batches = append(batches, addresses[i:end])
	}

	// Process batches with parallelization using errgroup
	var mu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for idx, batch := range batches {
		batchIdx := idx
		batchData := batch
		g.Go(func() error {
			// queryBatchBalances uses utils.Post which already handles retries with exponential backoff
			batchBalances, err := queryBatchBalances(ctx, jsonrpcURL, erc20Address, batchData, height)
			if err != nil {
				return fmt.Errorf("failed to query batch %d: %w", batchIdx, err)
			}

			// Merge batch results into main balances map
			mu.Lock()
			maps.Copy(balances, batchBalances)
			mu.Unlock()

			return nil
		})
	}

	// Wait for all goroutines and return first error if any
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return balances, nil
}

// queryBatchBalances queries balances for a batch of addresses at a specific height
func queryBatchBalances(ctx context.Context, jsonrpcURL string, erc20Address string, batch []richlistutils.AddressWithID, height int64) (map[richlistutils.AddressWithID]sdkmath.Int, error) {
	// balanceOf function selector: keccak256("balanceOf(address)")[:4] = 0x70a08231
	const balanceOfSelector = "0x70a08231"

	batchRequests := make([]JSONRPCRequest, 0, len(batch))
	batchRequests = append(batchRequests, JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_blockNumber",
		Params:  []any{},
		ID:      0,
	})

	// Create batch of JSON-RPC requests
	for idx, addrWithID := range batch {
		// Prepare the call data: balanceOf(address)
		// Format: 0x70a08231 + 000000000000000000000000 + address (without 0x)
		addressParam := strings.TrimPrefix(addrWithID.HexAddress, "0x")

		// Pad address to 32 bytes (64 hex chars) - efficient single allocation
		if len(addressParam) < 64 {
			addressParam = strings.Repeat("0", 64-len(addressParam)) + strings.TrimPrefix(addrWithID.HexAddress, "0x")
		}

		callData := balanceOfSelector + addressParam

		// Convert height to hex format (0x prefix)
		blockParam := fmt.Sprintf("0x%x", height)

		// Create JSON-RPC request with unique ID
		rpcReq := JSONRPCRequest{
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

	var batchResponses []JSONRPCResponse

	for attempt := 0; ; attempt++ {
		respBody, err := util.Post(ctx, jsonrpcURL, "", batchRequests, headers)
		if err != nil {
			return nil, fmt.Errorf("failed to send JSON-RPC batch request: %w", err)
		}

		// Parse batch response
		if err := json.Unmarshal(respBody, &batchResponses); err != nil {
			return nil, fmt.Errorf("failed to decode JSON-RPC batch response: %w", err)
		}

		// Process each response in the batch
		if len(batchResponses) != len(batch)+1 {
			return nil, fmt.Errorf("batch response count mismatch: expected %d, got %d", len(batch)+1, len(batchResponses))
		}

		// Parse the latest height from the batch response
		latestHeight, err := parseLatestHeightFromBatch(batchResponses)
		if err != nil {
			return nil, err
		}

		// Check if the latest height is less than the requested height
		if latestHeight < height {
			// Abort if context is done instead of sleeping indefinitely
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// Sleep before retrying with exponential backoff
			richlistutils.ExponentialBackoff(attempt)
			continue
		}

		// Height is sufficient, break out of retry loop
		break
	}

	// Build a map from request ID to the corresponding AddressWithID
	idToAddr := make(map[int]richlistutils.AddressWithID, len(batch))
	for idx, addrWithID := range batch {
		requestID := idx + 1
		idToAddr[requestID] = addrWithID
	}

	balances := make(map[richlistutils.AddressWithID]sdkmath.Int, len(batch))
	for _, rpcResp := range batchResponses {
		// Look up the address by response ID
		addrWithID, found := idToAddr[rpcResp.ID]
		if !found {
			return nil, fmt.Errorf("received response with unexpected ID %d", rpcResp.ID)
		}

		// Check for JSON-RPC error
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error for address %s: code=%d, message=%s", addrWithID.HexAddress, rpcResp.Error.Code, rpcResp.Error.Message)
		}

		// Parse balance from hex string
		balance, ok := richlistutils.ParseHexAmountToSDKInt(rpcResp.Result)
		if !ok {
			return nil, fmt.Errorf("failed to parse balance for address %s: %s", addrWithID.HexAddress, rpcResp.Result)
		}

		balances[addrWithID] = balance
	}

	return balances, nil
}

// parseLatestHeightFromBatch extracts and parses the latest block height from batch responses
func parseLatestHeightFromBatch(batchResponses []JSONRPCResponse) (int64, error) {
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
