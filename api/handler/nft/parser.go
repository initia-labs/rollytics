package nft

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/codec"
)

func ParseCollectionsRequest(c *fiber.Ctx) (*CollectionsRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}
	req := &CollectionsRequest{
		Pagination: pagination,
	}

	return req, nil
}

func ParseCollectionsByAccountRequest(c *fiber.Ctx) (*CollectionsByAccountRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &CollectionsByAccountRequest{
		Account:    c.Params("account"),
		Pagination: pagination,
	}

	if req.Account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}

	return req, nil
}

func ParseCollectionsByNameRequest(c *fiber.Ctx) (*CollectionsByNameRequest, error) {
	pagination, err := common.ExtractPaginationParams(c)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, common.ErrInvalidParams)
	}

	req := &CollectionsByNameRequest{
		Name:       c.Params("name"),
		Pagination: pagination,
	}

	if req.Name == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "name param is required")
	}

	return req, nil
}

func ParseCollectionByAddressRequest(c *fiber.Ctx) (*CollectionByAddrRequest, error) {
	collectionAddr := c.Params("collection_addr")
	if collectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr param is required")
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

	accAddr, err := codec.AccAddressFromString(account)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid account")
	}

	req := &TokensByAccountRequest{
		Account:    accAddr.String(),
		Pagination: pagination,
	}
	if req.Account == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "account param is required")
	}
	return req, nil
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

	req := &TokensByCollectionRequest{
		CollectionAddr: strings.ToLower(collectionAddr),
		Pagination:     pagination,
	}

	if req.CollectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr param is required")
	}

	return req, nil
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

	req := &NftTxsRequest{
		CollectionAddr: strings.ToLower(collectionAddr),
		TokenId:        c.Params("token_id"),
		Pagination:     pagination,
	}

	if req.CollectionAddr == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "collection_addr param is required")
	}
	if req.TokenId == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "token_id param is required")
	}

	return req, nil
}
