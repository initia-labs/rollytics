package nft

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

func ParseCollectionsRequest(c *fiber.Ctx) (*CollectionsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	return &CollectionsRequest{
		Pagination: pagination,
	}, nil
}

func ParseCollectionsByAccountRequest(c *fiber.Ctx) (*CollectionsByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	account := c.Params("account")
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}

	return &CollectionsByAccountRequest{
		Account:    account,
		Pagination: pagination,
	}, nil
}

func ParseCollectionsByNameRequest(c *fiber.Ctx) (*CollectionsByNameRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	name := c.Params("name")
	if name == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "name param is required")
	}

	return &CollectionsByNameRequest{
		Name:       name,
		Pagination: pagination,
	}, nil
}

func ParseCollectionByCollectionAddrRequest(config *config.ChainConfig, c *fiber.Ctx) (*CollectionByAddrRequest, error) {
	collectionAddr := c.Params("collection_addr")
	if collectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr param is required")
	}

	collectionAddr, err := validateCollectionAddr(config, collectionAddr)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid collection address : "+err.Error())
	}

	req := &CollectionByAddrRequest{
		CollectionAddr: strings.ToLower(collectionAddr),
	}

	return req, nil
}

// Tokens
func ParseTokensByAccountRequest(c *fiber.Ctx) (*TokensByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	account := c.Params("account")
	collectionAddr := c.Query("collection_addr")
	tokenId := c.Query("token_id")
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account is required")
	}

	accAddr, err := util.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account: "+err.Error())
	}

	return &TokensByAccountRequest{
		Account:        accAddr.String(),
		CollectionAddr: collectionAddr,
		TokenId:        tokenId,
		Pagination:     pagination,
	}, nil
}

func ParseTokensByCollectionRequest(config *config.ChainConfig, c *fiber.Ctx) (*TokensByCollectionRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	tokenId := c.Query("token_id")
	collectionAddr := c.Params("collection_addr")
	if collectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr is required")
	}

	collectionAddr, err = validateCollectionAddr(config, collectionAddr)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid collection address : "+err.Error())
	}
	return &TokensByCollectionRequest{
		CollectionAddr: collectionAddr,
		TokenId:        tokenId,
		Pagination:     pagination,
	}, nil
}

// txs
func ParseNftTxsRequest(config *config.ChainConfig, c *fiber.Ctx) (*NftTxsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	collectionAddr := c.Params("collection_addr")
	if collectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr is required")
	}

	collectionAddr, err = validateCollectionAddr(config, collectionAddr)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid collection address : "+err.Error())
	}

	tokenId := c.Params("token_id")
	if tokenId == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "token_id is required")
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
			return "", fiber.NewError(fiber.StatusBadRequest, "should be hex address starting with 0x")
		}
	case types.WasmVM:
		if !strings.HasPrefix(collectionAddr, config.AccountAddressPrefix) {
			return "", fiber.NewError(fiber.StatusBadRequest, "should be bech32 address with prefix "+config.AccountAddressPrefix)
		}
	}

	collectionAddr = strings.ToLower(collectionAddr)
	return collectionAddr, nil
}
