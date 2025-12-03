package querier

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/initia-labs/rollytics/types"
)

const (
	querySmartDataPath        = "/cosmwasm/wasm/v1/contract/%s/smart/%s"
	queryWasmContractDataPath = "/cosmwasm/wasm/v1/contract/%s"
)

var qreqContractInfo = base64.URLEncoding.EncodeToString([]byte("{\"contract_info\":{}}"))

func fetchContractInfo(collectionAddr string, height int64, timeout time.Duration) func(ctx context.Context, endpointURL string) (*types.QueryContractInfoResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.QueryContractInfoResponse, error) {
		body, err := querySmart(ctx, endpointURL, collectionAddr, qreqContractInfo, height, timeout)
		if err != nil {
			return nil, err
		}
		contractInfo, err := extractResponse[types.QueryContractInfoResponse](body)
		if err != nil {
			return nil, err
		}
		return &contractInfo, nil
	}
}

func fetchWasmContractInfo(collectionAddr string, height int64, timeout time.Duration) func(ctx context.Context, endpointURL string) (*types.QueryWasmContractResponse, error) {
	return func(ctx context.Context, endpointURL string) (*types.QueryWasmContractResponse, error) {
		body, err := queryWasmContract(ctx, endpointURL, collectionAddr, height, timeout)
		if err != nil {
			return nil, err
		}
		wasmContractInfo, err := extractResponse[types.QueryWasmContractResponse](body)
		if err != nil {
			return nil, err
		}
		return &wasmContractInfo, nil
	}
}
func (q *Querier) GetCollectionData(ctx context.Context, collectionAddr string, height int64) (name string, creator string, err error) {
	contractInfo, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchContractInfo(collectionAddr, height, queryTimeout))
	if err != nil {
		return "", "", fmt.Errorf("failed to query contract info: %w", err)
	}

	wasmContractInfo, err := executeWithEndpointRotation(ctx, q.RestUrls, fetchWasmContractInfo(collectionAddr, height, queryTimeout))
	if err != nil {
		return "", "", fmt.Errorf("failed to query wasm contract info: %w", err)
	}

	return contractInfo.Data.Name, wasmContractInfo.ContractInfo.Creator, nil
}

func querySmart(ctx context.Context, baseUrl, contractAddr, queryData string, height int64, timeout time.Duration) (response []byte, err error) {
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	return Get(ctx, baseUrl, fmt.Sprintf(querySmartDataPath, contractAddr, queryData), nil, headers, timeout)
}

func queryWasmContract(ctx context.Context, baseUrl, contractAddr string, height int64, timeout time.Duration) (response []byte, err error) {
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	return Get(ctx, baseUrl, fmt.Sprintf(queryWasmContractDataPath, contractAddr), nil, headers, timeout)
}
