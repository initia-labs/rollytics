package nft

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
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

func ParseCollectionByAddressRequest(c *fiber.Ctx) (*CollectionByAddrRequest, error) {
	collectionAddr := c.Params("collection_addr")
	if collectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr param is required")
	}

	collectionAddr, err := util.HexAddressFromString(collectionAddr)
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
	if account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account is required")
	}

	accAddr, err := util.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account")
	}

	return &TokensByAccountRequest{
		Account:    accAddr.String(),
		Pagination: pagination,
	}, nil
}

func ParseTokensByCollectionRequest(c *fiber.Ctx) (*TokensByCollectionRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	collectionAddr := c.Params("collection_addr")
	if collectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr is required")
	}

	collectionAddr, err = util.HexAddressFromString(collectionAddr)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid collection address : "+err.Error())
	}
	return &TokensByCollectionRequest{
		CollectionAddr: collectionAddr,
		Pagination:     pagination,
	}, nil
}

// txs
func ParseNftTxsRequest(c *fiber.Ctx) (*NftTxsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	collectionAddr := c.Params("collection_addr")
	if collectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr is required")
	}

	collectionAddr, err = util.HexAddressFromString(collectionAddr)
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
