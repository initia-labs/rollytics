package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/initia-labs/rollytics/types"
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
func fetchAllAccountsWithPagination(ctx context.Context, vmType types.VMType, restURL string, height int64) ([]sdk.AccAddress, error) {
	const path = "/cosmos/auth/v1beta1/accounts"

	var allAddresses []sdk.AccAddress
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
				if accAddress, err := util.AccAddressFromString(account.Address); err == nil {
					if vmType == types.EVM && len(accAddress) > 40 {
						continue
					}

					allAddresses = append(allAddresses, accAddress)
				}
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

// FetchMinterBurnerModuleAccounts fetches module accounts with "minter" or "burner" permissions
// from the Cosmos SDK blockchain using pagination. These accounts have the ability to mint or burn
// tokens and need to be excluded from rich list calculations to avoid incorrect total supply values.
func FetchMinterBurnerModuleAccounts(ctx context.Context, restURL string) ([]sdk.AccAddress, error) {
	const path = "/cosmos/auth/v1beta1/module_accounts"

	var moduleAccounts []sdk.AccAddress
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

		// Fetch page using util.Get (has built-in retry with exponential backoff)
		body, err := util.Get(ctx, restURL, path, params, nil)
		if err != nil {
			return nil, err
		}

		var accountsResp CosmosAccountsResponse
		if err := json.Unmarshal(body, &accountsResp); err != nil {
			return nil, err
		}

		// Filter accounts with minter or burner permissions
		for _, account := range accountsResp.Accounts {
			if account.Address != "" && (slices.Contains(account.Permissions, "minter") || slices.Contains(account.Permissions, "burner")) {
				if accAddress, err := util.AccAddressFromString(account.Address); err == nil {
					moduleAccounts = append(moduleAccounts, accAddress)
				}
			}
		}

		// Check if there are more pages
		if len(accountsResp.Pagination.NextKey) == 0 {
			// Workaround for broken API that returns null next_key prematurely
			if accountsResp.Pagination.Total != "" {
				total, err := strconv.Atoi(accountsResp.Pagination.Total)
				if err != nil {
					return moduleAccounts, fmt.Errorf("failed to parse pagination.total: %w", err)
				}

				if len(moduleAccounts) < total && len(accountsResp.Accounts) == paginationLimitInt {
					useOffset = true
					continue // try next page with offset
				}
			}
			break
		}

		nextKey = accountsResp.Pagination.NextKey
	}

	return moduleAccounts, nil
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
func fetchAccountBalancesWithPagination(ctx context.Context, restURL string, address sdk.AccAddress, height int64) ([]CosmosCoin, error) {
	path := fmt.Sprintf("/cosmos/bank/v1beta1/balances/%s", address.String())

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
		for _, balance := range balancesResp.Balances {
			var denom string
			if strings.HasPrefix(balance.Denom, "evm/") {
				denom = strings.ToLower(strings.ReplaceAll(balance.Denom, "evm/", "0x"))
			} else {
				denom = balance.Denom
			}
			allBalances = append(allBalances, CosmosCoin{
				Denom:  denom,
				Amount: balance.Amount,
			})
		}

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
