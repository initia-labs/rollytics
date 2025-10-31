package evmrichlist

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
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

	const batchSize = 10

	// Process addresses in batches
	for i := 0; i < len(addresses); i += batchSize {
		end := min(i+batchSize, len(addresses))

		batch := addresses[i:end]

		// Retry logic with exponential backoff
		var batchBalances map[richlistutils.AddressWithID]sdkmath.Int
		var err error

		for attempt := range richlistutils.MAX_ATTEMPTS {
			batchBalances, err = queryBatchBalances(ctx, jsonrpcURL, erc20Address, batch, height, i)
			if err == nil {
				break
			}

			// Exponential backoff before retry
			if attempt < richlistutils.MAX_ATTEMPTS-1 {
				richlistutils.ExponentialBackoff(attempt)
			}
		}

		if err != nil {
			return nil, fmt.Errorf("failed to query batch after %d attempts: %w", richlistutils.MAX_ATTEMPTS, err)
		}

		// Merge batch results into main balances map
		for addrWithID, balance := range batchBalances {
			balances[addrWithID] = balance
		}
	}

	return balances, nil
}

// queryBatchBalances queries balances for a batch of addresses at a specific height
func queryBatchBalances(ctx context.Context, jsonrpcURL string, erc20Address string, batch []richlistutils.AddressWithID, height int64, idOffset int) (map[richlistutils.AddressWithID]sdkmath.Int, error) {
	// balanceOf function selector: keccak256("balanceOf(address)")[:4] = 0x70a08231
	const balanceOfSelector = "0x70a08231"

	batchRequests := make([]JSONRPCRequest, 0, len(batch))

	// Create batch of JSON-RPC requests
	for idx, addrWithID := range batch {
		// Prepare the call data: balanceOf(address)
		// Format: 0x70a08231 + 000000000000000000000000 + address (without 0x)
		addressParam := addrWithID.Address
		if len(addressParam) >= 2 && addressParam[:2] == "0x" {
			addressParam = addressParam[2:]
		}

		// Pad address to 32 bytes (64 hex chars)
		for len(addressParam) < 64 {
			addressParam = "0" + addressParam
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
			ID: idOffset + idx, // Unique ID for each request in the batch
		}

		batchRequests = append(batchRequests, rpcReq)
	}

	// Send JSON-RPC batch request using util.Post
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	respBody, err := util.Post(ctx, jsonrpcURL, "", batchRequests, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to send JSON-RPC batch request: %w", err)
	}

	// Parse batch response
	var batchResponses []JSONRPCResponse
	if err := json.Unmarshal(respBody, &batchResponses); err != nil {
		return nil, fmt.Errorf("failed to decode JSON-RPC batch response: %w", err)
	}

	// Process each response in the batch
	if len(batchResponses) != len(batch) {
		return nil, fmt.Errorf("batch response count mismatch: expected %d, got %d", len(batch), len(batchResponses))
	}

	balances := make(map[richlistutils.AddressWithID]sdkmath.Int, len(batch))
	for idx, rpcResp := range batchResponses {
		addrWithID := batch[idx]

		// Check for JSON-RPC error
		if rpcResp.Error != nil {
			return nil, fmt.Errorf("JSON-RPC error for address %s: code=%d, message=%s", addrWithID.Address, rpcResp.Error.Code, rpcResp.Error.Message)
		}

		// Parse balance from hex string
		balance, ok := richlistutils.ParseHexAmountToSDKInt(rpcResp.Result)
		if !ok {
			return nil, fmt.Errorf("failed to parse balance for address %s: %s", addrWithID.Address, rpcResp.Result)
		}

		balances[addrWithID] = balance
	}

	return balances, nil
}
