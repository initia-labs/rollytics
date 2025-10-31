package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/initia-labs/rollytics/util"
)

const (
	paginationLimit = "100"
)

var paginationLimitInt int

func init() {
	var err error
	paginationLimitInt, err = strconv.Atoi(paginationLimit)
	if err != nil {
		panic(err)
	}
}

// fetchAllAccountsWithPagination queries all accounts from the Cosmos SDK REST API at a specific height.
// It paginates through all results using a limit of 100 per request until next_key is null.
// Similar to fetchAllTxsWithPagination, this uses util.Get which has built-in retry logic with exponential backoff.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - restURL: The Cosmos SDK REST API endpoint URL (e.g., "https://rest.example.com")
//   - height: The block height to query at
//
// Returns:
//   - A slice of account addresses
//   - error if the query fails
func fetchAllAccountsWithPagination(ctx context.Context, restURL string, height int64) ([]string, error) {
	const path = "/cosmos/auth/v1beta1/accounts"

	var allAddresses []string
	var nextKey []byte
	useOffset := false
	offset := 0

	for {
		// Build pagination parameters
		params := map[string]string{"pagination.limit": paginationLimit}
		if useOffset {
			params["pagination.offset"] = strconv.Itoa(offset)
		} else if len(nextKey) > 0 {
			params["pagination.key"] = base64.StdEncoding.EncodeToString(nextKey)
		}

		// Set height header for historical queries
		headers := map[string]string{
			"x-cosmos-block-height": fmt.Sprintf("%d", height),
		}

		// Fetch page using util.Get (has built-in retry with exponential backoff)
		body, err := util.Get(ctx, restURL, path, params, headers)
		if err != nil {
			return nil, err
		}

		var accountsResp CosmosAccountsResponse
		if err := json.Unmarshal(body, &accountsResp); err != nil {
			return nil, err
		}

		// Extract addresses from accounts
		for _, account := range accountsResp.Accounts {
			if account.Address != "" {
				allAddresses = append(allAddresses, account.Address)
			}
		}

		// Check if there are more pages
		if len(accountsResp.Pagination.NextKey) == 0 {
			// Workaround for broken API that returns null next_key prematurely.
			if accountsResp.Pagination.Total != "" {
				total, err := strconv.Atoi(accountsResp.Pagination.Total)
				if err != nil {
					return allAddresses, fmt.Errorf("failed to parse pagination.total: %w", err)
				}

				if len(allAddresses) < total && len(accountsResp.Accounts) == paginationLimitInt {
					useOffset = true
					continue // try next page with offset
				}
			}
			break
		}

		nextKey = accountsResp.Pagination.NextKey
	}

	return allAddresses, nil
}

// fetchAccountBalancesWithPagination queries all balances for a specific account address from the Cosmos SDK REST API at a specific height.
// It paginates through all results using a limit of 100 per request until next_key is null.
// Similar to fetchAllTxsWithPagination, this uses util.Get which has built-in retry logic with exponential backoff.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - restURL: The Cosmos SDK REST API endpoint URL (e.g., "https://rest.example.com")
//   - address: The account address to query balances for
//   - height: The block height to query at
//
// Returns:
//   - A slice of CosmosCoin containing all balances for the account
//   - error if the query fails
func fetchAccountBalancesWithPagination(ctx context.Context, restURL string, address string, height int64) ([]CosmosCoin, error) {
	path := fmt.Sprintf("/cosmos/bank/v1beta1/balances/%s", address)

	var allBalances []CosmosCoin
	var nextKey []byte
	useOffset := false
	offset := 0

	for {
		// Build pagination parameters
		params := map[string]string{"pagination.limit": paginationLimit}
		if useOffset {
			params["pagination.offset"] = strconv.Itoa(offset)
		} else if len(nextKey) > 0 {
			params["pagination.key"] = base64.StdEncoding.EncodeToString(nextKey)
		}

		// Set height header for historical queries
		headers := map[string]string{
			"x-cosmos-block-height": fmt.Sprintf("%d", height),
		}

		// Fetch page using util.Get (has built-in retry with exponential backoff)
		body, err := util.Get(ctx, restURL, path, params, headers)
		if err != nil {
			return nil, err
		}

		var balancesResp CosmosBalancesResponse
		if err := json.Unmarshal(body, &balancesResp); err != nil {
			return nil, err
		}

		// Append balances from this page
		allBalances = append(allBalances, balancesResp.Balances...)

		// Check if there are more pages
		// Check if there are more pages
		if len(balancesResp.Pagination.NextKey) == 0 {
			// Workaround for broken API that returns null next_key prematurely.
			if balancesResp.Pagination.Total != "" {
				total, err := strconv.Atoi(balancesResp.Pagination.Total)
				if err != nil {
					return allBalances, fmt.Errorf("failed to parse pagination.total: %w", err)
				}

				if len(allBalances) < total && len(balancesResp.Balances) == paginationLimitInt {
					useOffset = true
					continue // try next page with offset
				}
			}
			break
		}
		nextKey = balancesResp.Pagination.NextKey
	}

	return allBalances, nil
}
