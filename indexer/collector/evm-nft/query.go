package evm_nft

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/initia-labs/minievm/x/evm/contracts/erc721"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func getCollectionName(collectionAddr string, cfg *config.Config, height int64) (name string, err error) {
	abi, err := erc721.Erc721MetaData.GetAbi()
	if err != nil {
		return name, err
	}

	input, err := abi.Pack("name")
	if err != nil {
		return name, err
	}

	callRes, err := evmCall(collectionAddr, input, cfg, height)
	if err != nil {
		return name, err
	}

	err = abi.UnpackIntoInterface(&name, "name", callRes)
	return
}

func getTokenUri(collectionAddr, tokenIdStr string, cfg *config.Config, height int64) (tokenUri string, err error) {
	abi, err := erc721.Erc721MetaData.GetAbi()
	if err != nil {
		return tokenUri, err
	}

	tokenId, ok := new(big.Int).SetString(tokenIdStr, 10)
	if !ok {
		return tokenUri, types.NewInvalidValueError("token_id", tokenIdStr, "must be a valid decimal number")
	}
	input, err := abi.Pack("tokenURI", tokenId)
	if err != nil {
		return tokenUri, err
	}

	callRes, err := evmCall(collectionAddr, input, cfg, height)
	if err != nil {
		return tokenUri, err
	}

	err = abi.UnpackIntoInterface(&tokenUri, "tokenURI", callRes)
	return
}

func evmCall(contractAddr string, input []byte, cfg *config.Config, height int64) (response []byte, err error) {
	payload := map[string]interface{}{
		"sender":        "init1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqpqr5e3d",
		"contract_addr": contractAddr,
		"input":         fmt.Sprintf("0x%s", hex.EncodeToString(input)),
		"value":         "0",
	}
	headers := map[string]string{"x-cosmos-block-height": fmt.Sprintf("%d", height)}
	path := "/minievm/evm/v1/call"
	ctx, cancel := context.WithTimeout(context.Background(), cfg.GetQueryTimeout())
	defer cancel()
	body, err := util.Post(ctx, cfg.GetChainConfig().RestUrl, path, payload, headers)
	if err != nil {
		return response, err
	}

	var callRes QueryCallResponse
	if err := json.Unmarshal(body, &callRes); err != nil {
		return response, err
	}

	if callRes.Error != "" {
		return response, fmt.Errorf("error from evm call: %s", callRes.Error)
	}

	return hex.DecodeString(strings.TrimPrefix(callRes.Response, "0x"))
}
