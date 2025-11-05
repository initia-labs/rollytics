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
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/initia-labs/rollytics/config"
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
func fetchAllAccountsWithPagination(ctx context.Context, cfg *config.Config, height int64) ([]sdk.AccAddress, error) {
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
		body, err := util.Get(ctx, cfg.GetChainConfig().RestUrl, path, params, headers)
		if err != nil {
			return nil, err
		}

		var accountsResp QueryAccountsResponse
		if err := json.Unmarshal(body, &accountsResp); err != nil {
			return nil, err
		}

		// Extract addresses from accounts
		for _, account := range accountsResp.Accounts {
			if account == nil {
				continue
			}

			addr, err := getAddressFromAccount(account)
			if err != nil {
				continue
			}

			if addr != "" {
				if accAddress, err := util.AccAddressFromString(addr); err == nil {
					if cfg.GetVmType() == types.EVM && len(accAddress) > 20 {
						continue
					}
					allAddresses = append(allAddresses, accAddress)
				}
			}
		}

		// Check if there are more pages
		var nextKeyBytes []byte
		if accountsResp.Pagination != nil {
			nextKeyBytes = accountsResp.Pagination.NextKey
		}
		if len(nextKeyBytes) == 0 {
			// Workaround for broken API that returns null next_key prematurely.
			if accountsResp.Pagination != nil && accountsResp.Pagination.Total != "" {
				total, err := strconv.Atoi(accountsResp.Pagination.Total)
				if err != nil {
					return nil, fmt.Errorf("failed to parse pagination.total: %w", err)
				}
				if len(allAddresses) < total && len(accountsResp.Accounts) == paginationLimitInt {
					useOffset = true
					continue // try next page with offset
				}
			}
			break
		}

		nextKey = nextKeyBytes
	}

	return allAddresses, nil
}

// FetchMinterBurnerModuleAccounts fetches module accounts with "minter" or "burner" permissions
// from the Cosmos SDK blockchain using pagination. These accounts have the ability to mint or burn
// tokens and need to be excluded from rich list calculations to avoid incorrect total supply values.
func FetchMinterBurnerModuleAccounts(ctx context.Context, restURL string) ([]sdk.AccAddress, error) {
	const path = "/cosmos/auth/v1beta1/module_accounts"

	var moduleAccounts []sdk.AccAddress

	// Fetch page using util.Get (has built-in retry with exponential backoff)
	body, err := util.Get(ctx, restURL, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var accountsResp authtypes.QueryModuleAccountsResponse
	if err := json.Unmarshal(body, &accountsResp); err != nil {
		return nil, err
	}

	// Filter accounts with minter or burner permissions
	for _, account := range accountsResp.Accounts {
		if account == nil {
			continue
		}

		var moduleAccount authtypes.ModuleAccount
		if err := json.Unmarshal(account.Value, &moduleAccount); err != nil {
			continue
		}

		if moduleAccount.Address != "" && (slices.Contains(moduleAccount.Permissions, "minter") || slices.Contains(moduleAccount.Permissions, "burner")) {
			if accAddress, err := util.AccAddressFromString(moduleAccount.Address); err == nil {
				moduleAccounts = append(moduleAccounts, accAddress)
			}
		}
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
func fetchAccountBalancesWithPagination(ctx context.Context, cfg *config.Config, address sdk.AccAddress, height int64) ([]CosmosCoin, error) {
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
		body, err := util.Get(ctx, cfg.GetChainConfig().RestUrl, path, params, headers)
		if err != nil {
			return nil, err
		}

		var balancesResp QueryAllBalancesResponse
		if err := json.Unmarshal(body, &balancesResp); err != nil {
			return nil, err
		}

		// Append balances from this page
		for _, balance := range balancesResp.Balances {
			denom := strings.ToLower(balance.Denom)
			if cfg.GetVmType() == types.EVM {
				contract, err := util.GetEvmContractByDenom(ctx, denom)
				if err != nil {
					continue
				}
				denom = contract
			}

			allBalances = append(allBalances, CosmosCoin{
				Denom:  denom,
				Amount: balance.Amount.String(),
			})
		}

		// Check if there are more pages
		var nextKeyBytes []byte
		if balancesResp.Pagination != nil {
			nextKeyBytes = balancesResp.Pagination.NextKey
		}
		if len(nextKeyBytes) == 0 {
			// Workaround for broken API that returns null next_key prematurely.
			if balancesResp.Pagination != nil && balancesResp.Pagination.Total != "" {
				total, err := strconv.Atoi(balancesResp.Pagination.Total)
				if err != nil {
					return nil, fmt.Errorf("failed to parse pagination.total: %w", err)
				}
				if len(allBalances) < total && len(balancesResp.Balances) == paginationLimitInt {
					useOffset = true
					continue // try next page with offset
				}
			}
			break
		}

		nextKey = nextKeyBytes
	}

	return allBalances, nil
}
