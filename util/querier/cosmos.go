package querier

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

const (
	paginationLimitInt       = 100
	paginationLimit          = "100"
	cosmosTxsPath            = "/cosmos/tx/v1beta1/txs/block/%d"
	cosmosLatestHeightPath   = "/cosmos/base/tendermint/v1beta1/blocks/latest"
	cosmosModuleAccountsPath = "/cosmos/auth/v1beta1/module_accounts"
	cosmosBalancesPath       = "/cosmos/bank/v1beta1/balances/%s"
	cosmosAccountsPath       = "/cosmos/auth/v1beta1/accounts"
	cosmosNodeInfoPath       = "/cosmos/base/tendermint/v1beta1/node_info"
)

// handlePaginationNextKey handles pagination logic for broken APIs that return null next_key prematurely.
// Returns (shouldContinue, shouldBreak) where:
// - shouldContinue: true if we should continue with offset pagination
// - shouldBreak: true if we should break (no more pages)
func handlePaginationNextKey(pagination types.Pagination, currentCount, pageSize int, useOffset *bool) (bool, bool) {
	if len(pagination.NextKey) != 0 {
		return false, false // Continue with next key
	}

	// Workaround for broken API that returns null next_key prematurely.
	if pagination.Total == "" {
		return false, true // No more pages
	}

	total, err := strconv.Atoi(pagination.Total)
	if err != nil {
		return false, true // Can't parse total, assume done
	}

	if currentCount < total && pageSize == paginationLimitInt {
		*useOffset = true
		return true, false // Continue with offset
	}

	return false, true // No more pages
}

func (q *Querier) GetCosmosTxs(ctx context.Context, height int64, txCount int) (txs []types.RestTx, err error) {
	for {
		allTxs, err := q.fetchAllTxsWithPagination(ctx, height)
		if err != nil {
			return txs, err
		}

		// If we get the expected number of transactions, return immediately
		if len(allTxs) == txCount {
			return allTxs, nil
		}
	}
}

func fetchTxs(height int64, useOffset bool, offset int, nextKey []byte, timeout time.Duration) func(ctx context.Context, endpointURL string) (*types.QueryRestTxsResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.QueryRestTxsResponse, error) {
		params := map[string]string{"pagination.limit": paginationLimit}
		if useOffset {
			params["pagination.offset"] = strconv.Itoa(offset)
		} else if len(nextKey) > 0 {
			params["pagination.key"] = base64.StdEncoding.EncodeToString(nextKey)
		}
		body, err := Get(ctx, endpointURL, fmt.Sprintf(cosmosTxsPath, height), params, nil, timeout)
		if err != nil {
			return nil, err
		}
		var response types.QueryRestTxsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, err
		}
		return &response, nil
	}
}

func (q *Querier) fetchAllTxsWithPagination(ctx context.Context, height int64) ([]types.RestTx, error) {
	var allTxs []types.RestTx
	var nextKey []byte
	useOffset := false
	offset := 0

	for {
		requestFn := fetchTxs(height, useOffset, offset, nextKey, queryTimeout)
		response, err := executeWithEndpointRotation(ctx, q.RestUrls, requestFn)
		if err != nil {
			return allTxs, err
		}

		allTxs = append(allTxs, response.Txs...)
		offset = len(allTxs)

		shouldContinue, shouldBreak := handlePaginationNextKey(response.Pagination, len(allTxs), len(response.Txs), &useOffset)
		if shouldBreak {
			break // No more pages
		}
		if shouldContinue {
			continue // try next page with offset
		}

		nextKey = response.Pagination.NextKey
	}

	return allTxs, nil
}

func fetchLatestHeight() func(ctx context.Context, endpointURL string) (*types.BlockResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.BlockResponse, error) {
		body, err := Get(ctx, endpointURL, cosmosLatestHeightPath, nil, nil, queryTimeout)
		if err != nil {
			return nil, err
		}
		res, err := extractResponse[types.BlockResponse](body)
		if err != nil {
			return nil, err
		}
		return &res, nil
	}
}
func (q *Querier) GetLatestHeight(ctx context.Context) (int64, error) {
	res, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchLatestHeight())
	if err != nil {
		return 0, err
	}

	height := int64(0)
	if _, err := fmt.Sscanf(res.Block.Header.Height, "%d", &height); err != nil {
		return 0, types.NewInvalidValueError("height", res.Block.Header.Height, "failed to parse as integer")
	}

	return height, nil
}

func fetchValidator(validatorAddr string) func(ctx context.Context, endpointURL string) (*types.ValidatorResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.ValidatorResponse, error) {
		body, err := Get(ctx, endpointURL, fmt.Sprintf("/opinit/opchild/v1/validator/%s", validatorAddr), nil, nil, queryTimeout)
		if err != nil {
			return nil, err
		}
		response, err := extractResponse[types.ValidatorResponse](body)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}
}

func (q *Querier) GetValidator(ctx context.Context, validatorAddr string) (*types.ValidatorResponse, error) {
	res, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchValidator(validatorAddr))
	if err != nil {
		return nil, err
	}
	return res, nil
}

func fetchMinterBurnerModuleAccounts() func(ctx context.Context, endpointURL string) (*types.QueryModuleAccountsResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.QueryModuleAccountsResponse, error) {
		body, err := Get(ctx, endpointURL, cosmosModuleAccountsPath, nil, nil, queryTimeout)
		if err != nil {
			return nil, err
		}

		accountsResp, err := extractResponse[types.QueryModuleAccountsResponse](body)
		if err != nil {
			return nil, err
		}

		return &accountsResp, nil
	}
}
func (q *Querier) GetMinterBurnerModuleAccounts(ctx context.Context) ([]sdk.AccAddress, error) {
	res, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchMinterBurnerModuleAccounts())
	if err != nil {
		return nil, err
	}

	var moduleAccounts []sdk.AccAddress
	// Filter accounts with minter or burner permissions
	for _, account := range res.Accounts {
		if account.Address != "" && (slices.Contains(account.Permissions, "minter") || slices.Contains(account.Permissions, "burner")) {
			if accAddress, err := util.AccAddressFromString(account.Address); err == nil {
				moduleAccounts = append(moduleAccounts, accAddress)
			}
		}
	}

	return moduleAccounts, nil
}

func fetchCosmosBalances(address sdk.AccAddress, height int64, useOffset bool, nextKey []byte) func(ctx context.Context, endpointURL string) (*types.QueryAllBalancesResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.QueryAllBalancesResponse, error) {
		// Build pagination parameters
		params := map[string]string{"pagination.limit": paginationLimit}
		if useOffset {
			params["pagination.offset"] = "0"
		} else if len(nextKey) > 0 {
			params["pagination.key"] = base64.StdEncoding.EncodeToString(nextKey)
		}
		headers := map[string]string{
			"x-cosmos-block-height": fmt.Sprintf("%d", height),
		}
		body, err := Get(ctx, endpointURL, fmt.Sprintf(cosmosBalancesPath, address.String()), params, headers, queryTimeout)
		if err != nil {
			return nil, err
		}

		balancesResp, err := extractResponse[types.QueryAllBalancesResponse](body)
		if err != nil {
			return nil, err
		}
		return &balancesResp, nil
	}
}

func (q *Querier) GetAllBalances(ctx context.Context, address sdk.AccAddress, height int64) ([]sdk.Coin, error) {
	var allBalances []sdk.Coin
	var nextKey []byte
	useOffset := false

	for {
		balancesResp, err := executeWithEndpointRotation(
			ctx, q.RestUrls,
			fetchCosmosBalances(address, height, useOffset, nextKey),
		)
		if err != nil {
			return nil, err
		}

		// Append balances from this page
		for _, balance := range balancesResp.Balances {
			denom := strings.ToLower(balance.Denom)
			if q.VmType == types.EVM {
				contract, err := q.GetEvmContractByDenom(ctx, denom)
				if err != nil {
					return nil, err
				}
				denom = contract
			}

			allBalances = append(allBalances, sdk.Coin{
				Denom:  denom,
				Amount: balance.Amount,
			})
		}
		// Check if there are more pages
		shouldContinue, shouldBreak := handlePaginationNextKey(balancesResp.Pagination, len(allBalances), len(balancesResp.Balances), &useOffset)
		if shouldBreak {
			break
		}
		if shouldContinue {
			continue // try next page with offset
		}
		nextKey = balancesResp.Pagination.NextKey
	}
	return allBalances, nil
}

func fetchAllAccountsWithPagination(height int64, useOffset bool, nextKey []byte) func(ctx context.Context, endpointURL string) (*types.CosmosAccountsResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.CosmosAccountsResponse, error) {
		// Build pagination parameters
		params := map[string]string{"pagination.limit": paginationLimit}
		if useOffset {
			params["pagination.offset"] = "0"
		} else if len(nextKey) > 0 {
			params["pagination.key"] = base64.StdEncoding.EncodeToString(nextKey)
		}

		headers := map[string]string{
			"x-cosmos-block-height": fmt.Sprintf("%d", height),
		}
		body, err := Get(ctx, endpointURL, cosmosAccountsPath, params, headers, queryTimeout)
		if err != nil {
			return nil, err
		}
		var accountsResp types.CosmosAccountsResponse
		if err := json.Unmarshal(body, &accountsResp); err != nil {
			return nil, err
		}
		return &accountsResp, nil
	}
}

func (q *Querier) FetchAllAccountsWithPagination(ctx context.Context, height int64) ([]sdk.AccAddress, error) {
	var allAddresses []sdk.AccAddress
	var nextKey []byte
	useOffset := false

	for {
		accountsResp, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchAllAccountsWithPagination(height, useOffset, nextKey))
		if err != nil {
			return nil, err
		}

		extracted := q.extractAddressesFromAccounts(accountsResp.Accounts)
		allAddresses = append(allAddresses, extracted...)

		// Check if there are more pages
		shouldContinue, shouldBreak := handlePaginationNextKey(accountsResp.Pagination, len(allAddresses), len(accountsResp.Accounts), &useOffset)
		if shouldBreak {
			break
		}
		if shouldContinue {
			continue // try next page with offset
		}

		nextKey = accountsResp.Pagination.NextKey
	}

	return allAddresses, nil
}

func (q *Querier) extractAddressesFromAccounts(accounts []types.CosmosAccount) []sdk.AccAddress {
	var addresses []sdk.AccAddress
	for _, account := range accounts {
		if account.Address == "" {
			continue
		}
		accAddress, err := util.AccAddressFromString(account.Address)
		if err != nil {
			continue
		}
		if q.VmType == types.EVM && len(accAddress) > 20 {
			continue
		}
		addresses = append(addresses, accAddress)
	}
	return addresses
}

func fetchNodeInfo() func(ctx context.Context, endpointURL string) (*types.NodeInfo, error) {
	return func(ctx context.Context, endpointURL string) (*types.NodeInfo, error) {
		body, err := Get(ctx, endpointURL, cosmosNodeInfoPath, nil, nil, queryTimeout)
		if err != nil {
			return nil, err
		}
		nodeInfo, err := extractResponse[types.NodeInfo](body)
		if err != nil {
			return nil, err
		}
		return &nodeInfo, nil
	}
}

func (q *Querier) GetNodeInfo(ctx context.Context) (*types.NodeInfo, error) {
	res, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchNodeInfo())
	if err != nil {
		return nil, err
	}
	return res, nil
}
