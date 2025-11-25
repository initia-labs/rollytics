package wasm_nft

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util"
)

var (
	qreqContractInfo          = base64.URLEncoding.EncodeToString([]byte("{\"contract_info\":{}}"))
	querySmartDataPath        = "/cosmwasm/wasm/v1/contract/%s/smart/%s"
	queryWasmContractDataPath = "/cosmwasm/wasm/v1/contract/%s"
)

func extractResponse[T any](response []byte) (T, error) {
	var t T
	if err := json.Unmarshal(response, &t); err != nil {
		return t, err
	}
	return t, nil
}

func GetCollectionData(collectionAddr string, cfg *config.Config, height int64) (name string, creator string, err error) {
	// Query contract info
	contractInfo, err := queryContractInfo(collectionAddr, cfg, height)
	if err != nil {
		return "", "", fmt.Errorf("failed to query contract info: %w", err)
	}

	// Query wasm contract info
	wasmContractInfo, err := queryWasmContractInfo(collectionAddr, cfg, height)
	if err != nil {
		return "", "", fmt.Errorf("failed to query wasm contract info: %w", err)
	}

	return contractInfo.Data.Name, wasmContractInfo.ContractInfo.Creator, nil
}

func queryContractInfo(collectionAddr string, cfg *config.Config, height int64) (*QueryContractInfoResponse, error) {
	body, err := querySmart(collectionAddr, qreqContractInfo, cfg, height)
	if err != nil {
		return nil, err
	}
	contractInfo, err := extractResponse[QueryContractInfoResponse](body)
	if err != nil {
		return nil, err
	}

	return &contractInfo, nil
}

func queryWasmContractInfo(collectionAddr string, cfg *config.Config, height int64) (*QueryWasmContractResponse, error) {
	body, err := queryWasmContract(collectionAddr, cfg, height)
	if err != nil {
		return nil, err
	}

	wasmContractInfo, err := extractResponse[QueryWasmContractResponse](body)
	if err != nil {
		return nil, err
	}

	return &wasmContractInfo, nil
}

func querySmart(contractAddr, queryData string, cfg *config.Config, height int64) (response []byte, err error) {
	path := fmt.Sprintf(querySmartDataPath, contractAddr, queryData)
	return util.Get(context.Background(), cfg.GetChainConfig().RestUrl, path, nil, map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}, cfg.GetQueryTimeout())
}

func queryWasmContract(contractAddr string, cfg *config.Config, height int64) (response []byte, err error) {
	path := fmt.Sprintf(queryWasmContractDataPath, contractAddr)
	return util.Get(context.Background(), cfg.GetChainConfig().RestUrl, path, nil, map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}, cfg.GetQueryTimeout())
}
