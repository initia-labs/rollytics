package evmrichlist

import (
	"context"
	"fmt"
	"maps"
	"sync"

	sdkmath "cosmossdk.io/math"
	"golang.org/x/sync/errgroup"

	"github.com/initia-labs/rollytics/config"
	richlistutils "github.com/initia-labs/rollytics/indexer/extension/richlist/utils"
	"github.com/initia-labs/rollytics/util/querier"
)

const MAX_RETRY_ATTEMPTS_BEFORE_SENTRY = 10

// queryERC20Balances queries the balances of multiple addresses for a specific ERC20 token via JSON-RPC.
// It returns a map of AddressWithID to balance (as sdkmath.Int).
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - cfg: Configuration
//   - erc20Address: The ERC20 token contract address (with 0x prefix)
//   - addresses: List of addresses with account IDs to query
//   - height: The block height to query at
//
// The function uses the eth_call method to call the balanceOf function on the ERC20 contract.
// The balanceOf function signature is: balanceOf(address) returns (uint256)
// Function selector: 0x70a08231
func queryERC20Balances(ctx context.Context, cfg *config.Config, erc20Address string, addresses []richlistutils.AddressWithID, height int64) (map[richlistutils.AddressWithID]sdkmath.Int, error) {
	if len(addresses) == 0 {
		return make(map[richlistutils.AddressWithID]sdkmath.Int), nil
	}

	balances := make(map[richlistutils.AddressWithID]sdkmath.Int, len(addresses))

	const batchSize = 500
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
			// TODO: revisit
			batchBalances, err := queryBatchBalances(ctx, querier.NewQuerier(cfg.GetChainConfig()), erc20Address, batchData, height)
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
func queryBatchBalances(ctx context.Context, querier *querier.Querier, erc20Address string, batch []richlistutils.AddressWithID, height int64) (map[richlistutils.AddressWithID]sdkmath.Int, error) {
	addresses := make([]string, 0, len(batch))
	for _, addrWithID := range batch {
		addresses = append(addresses, addrWithID.HexAddress)
	}

	batchResponses, err := querier.QueryERC20Balances(ctx, erc20Address, addresses, height)
	if err != nil {
		return nil, err
	}

	// Build a map from request ID to the corresponding AddressWithID
	idToAddr := make(map[int]richlistutils.AddressWithID, len(batch))
	for idx, addrWithID := range batch {
		idToAddr[idx+1] = addrWithID
	}

	balances := make(map[richlistutils.AddressWithID]sdkmath.Int, len(batch))
	for _, rpcResp := range batchResponses {
		// Skip the eth_blockNumber response (ID 0)
		if rpcResp.ID == 0 {
			continue
		}

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
