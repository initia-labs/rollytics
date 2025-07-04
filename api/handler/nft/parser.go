package nft

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
)

func ParseCollectionsRequest(c *fiber.Ctx) *CollectionsRequest {
	pagination := common.ExtractPaginationParams(c)

	return &CollectionsRequest{
		Pagination: pagination,
	}
}

func ParseCollectionsByAccountRequest(c *fiber.Ctx) (*CollectionsByAccountRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	account, err := common.GetParams(c, "account")
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return &CollectionsByAccountRequest{
		Account:    account,
		Pagination: pagination,
	}, nil
}

func ParseCollectionsByNameRequest(c *fiber.Ctx) (*CollectionsByNameRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	name, err := common.GetParams(c, "name")
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return &CollectionsByNameRequest{
		Name:       name,
		Pagination: pagination,
	}, nil
}

func ParseCollectionByCollectionAddrRequest(config *config.ChainConfig, c *fiber.Ctx) (*CollectionByAddrRequest, error) {
	collectionAddr, err := getCollectionAddrParam(c, config)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	req := &CollectionByAddrRequest{
		CollectionAddr: strings.ToLower(collectionAddr),
	}

	return req, nil
}

// Tokens
func ParseTokensByAccountRequest(c *fiber.Ctx) (*TokensByAccountRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	accAddr, err := common.GetAccountParam(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return &TokensByAccountRequest{
		Account:        accAddr.String(),
		CollectionAddr: c.Query("collection_addr"),
		TokenId:        c.Query("token_id"),
		Pagination:     pagination,
	}, nil
}

func ParseTokensByCollectionRequest(config *config.ChainConfig, c *fiber.Ctx) (*TokensByCollectionRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	collectionAddr, err := getCollectionAddrParam(c, config)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return &TokensByCollectionRequest{
		CollectionAddr: collectionAddr,
		TokenId:        c.Query("token_id"),
		Pagination:     pagination,
	}, nil
}

// txs
func ParseNftTxsRequest(config *config.ChainConfig, c *fiber.Ctx) (*NftTxsRequest, error) {
	pagination := common.ExtractPaginationParams(c)
	collectionAddr, err := getCollectionAddrParam(c, config)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	tokenId, err := common.GetParams(c, "token_id")
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return &NftTxsRequest{
		CollectionAddr: collectionAddr,
		TokenId:        tokenId,
		Pagination:     pagination,
	}, nil
}

func validateCollectionAddr(config *config.ChainConfig, collectionAddr string) (string, error) {
	switch config.VmType {
	case types.MoveVM, types.EVM:
		if !strings.HasPrefix(collectionAddr, "0x") {
			return "", fiber.NewError(fiber.StatusBadRequest, "collection addr should be hex address")
		}
	case types.WasmVM:
		if !strings.HasPrefix(collectionAddr, config.AccountAddressPrefix) {
			return "", fiber.NewError(fiber.StatusBadRequest, "collection addr should be bech32 address")
		}
	}

	collectionAddr = strings.ToLower(collectionAddr)
	return collectionAddr, nil
}

func getCollectionAddrParam(c *fiber.Ctx, config *config.ChainConfig) (string, error) {
	collectionAddr, err := common.GetParams(c, "collection_addr")
	if err != nil {
		return "", fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	collectionAddr, err = validateCollectionAddr(config, collectionAddr)
	if err != nil {
		return "", fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid collection address: %s", err.Error()))
	}
	return collectionAddr, nil
}
