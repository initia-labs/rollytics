package tx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

const (
	paginationLimit = "100"
	maxRetries      = 10
	retryDelay      = 500 * time.Millisecond
)

var paginationLimitInt int

func init() {
	var err error
	paginationLimitInt, err = strconv.Atoi(paginationLimit)
	if err != nil {
		panic(err)
	}
}

// getCosmosTxs retrieves all transactions for a specific block height with retry logic.
//
// This function implements a retry mechanism because there can be a brief window where:
// - The block data (via RPC) shows that transactions exist (txCount > 0)
// - But the REST API query returns empty results temporarily
//
// This happens because different endpoints may have slightly different data propagation
// timing in the node, creating a temporary inconsistency where block metadata is available
// but transaction details are not yet queryable via REST API.
//
// The retry logic with 500ms delays helps handle this temporary state until the data
// becomes consistent across all endpoints.
func getCosmosTxs(cfg *config.Config, height int64, txCount int) (txs []RestTx, err error) {
	path := fmt.Sprintf("/cosmos/tx/v1beta1/txs/block/%d", height)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		allTxs, err := fetchAllTxsWithPagination(cfg, path)
		if err != nil {
			return txs, err
		}

		// If we get the expected number of transactions, return immediately
		if len(allTxs) == txCount {
			return allTxs, nil
		}

		// If this is not the last attempt and we got an empty result, wait and retry
		// This specifically handles the case where block shows txCount > 0 but REST API returns no txs
		if attempt < maxRetries && len(allTxs) == 0 {
			time.Sleep(retryDelay)
			continue
		}

		// If we got some transactions but not the expected count, or this is the last attempt
		return txs, fmt.Errorf("expected %d txs but got %d for height %d (attempt %d/%d)",
			txCount, len(allTxs), height, attempt+1, maxRetries+1)
	}

	return txs, fmt.Errorf("failed to get cosmos txs after %d retries for height %d", maxRetries+1, height)
}

func fetchAllTxsWithPagination(cfg *config.Config, path string) ([]RestTx, error) {
	var allTxs []RestTx
	var nextKey []byte

	for {
		params := map[string]string{"pagination.limit": paginationLimit}
		if len(nextKey) > 0 {
			params["pagination.key"] = base64.StdEncoding.EncodeToString(nextKey)
		}

		ctx, cancel := context.WithTimeout(context.Background(), cfg.GetQueryTimeout())
		defer cancel() // defer is safe here as tx pagination very rarely requires many iterations
		body, err := util.Get(ctx, cfg.GetChainConfig().RestUrl, path, params, nil)

		if err != nil {
			return allTxs, err
		}

		var response QueryRestTxsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return allTxs, err
		}

		allTxs = append(allTxs, response.Txs...)

		if len(response.Pagination.NextKey) == 0 || len(response.Txs) < paginationLimitInt {
			break
		}

		nextKey = response.Pagination.NextKey
	}

	return allTxs, nil
}

func getEvmTxs(cfg *config.Config, height int64) (txs []types.EvmTx, err error) {
	if cfg.GetVmType() != types.EVM {
		return
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockReceipts",
		"params":  []string{fmt.Sprintf("0x%x", height)},
		"id":      1,
	}
	headers := map[string]string{"Content-Type": "application/json"}
	path := ""

	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetQueryTimeout())
	defer cancel()
	body, err := util.Post(ctx, cfg.GetChainConfig().JsonRpcUrl, path, payload, headers)
	if err != nil {
		return txs, err
	}

	var res QueryEvmTxsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return txs, err
	}

	return res.Result, nil
}
