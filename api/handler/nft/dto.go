package nft

import (
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

// Colections
// Request
type CollectionsRequest struct {
	Pagination *common.PaginationParams `query:"pagination"`
}

type CollectionsByAccountRequest struct {
	Account    string                   `param:"account" extensions:"x-order:0"`
	Pagination *common.PaginationParams `query:"pagination" extensions:"x-order:1"`
}

type CollectionsByNameRequest struct {
	Pagination *common.PaginationParams `query:"pagination" extensions:"x-order:0"`
	Name       string                   `param:"name" extensions:"x-order:1"`
}

type CollectionByAddrRequest struct {
	CollectionAddr string `param:"collection_addr"`
}

// Response
type Collection struct {
	Creator    string `json:"creator" extensions:"x-order:0"`
	Address    string `json:"address" extensions:"x-order:1"`
	Name       string `json:"name" extensions:"x-order:2"`
	OriginName string `json:"origin_name" extensions:"x-order:3"`
	NFTCount   int64  `json:"nft_count" extensions:"x-order:4"`
}

type CollectionsResponse struct {
	Collections []Collection         `json:"collections" extensions:"x-order:0"`
	Pagination  *common.PageResponse `json:"pagination" extensions:"x-order:1"`
}

type CollectionResponse struct {
	Collection *Collection `json:"collection"`
}

func ToResponseCollection(col *types.CollectedNftCollection) *Collection {
	return &Collection{
		Creator:    col.Creator,
		Address:    col.Addr,
		Name:       col.Name,
		OriginName: col.OriginName,
		NFTCount:   col.NftCount,
	}
}

func BatchToResponseCollections(cols []types.CollectedNftCollection) []Collection {
	collections := make([]Collection, 0, len(cols))
	for _, col := range cols {
		collections = append(collections, *ToResponseCollection(&col))
	}
	return collections
}

// Tokens
// Request
type TokensByAccountRequest struct {
	Account    string                   `param:"account" extensions:"x-order:0"`
	Pagination *common.PaginationParams `query:"pagination" extensions:"x-order:1"`
}

type TokensByCollectionRequest struct {
	CollectionAddr string                   `param:"collection_addr" extensions:"x-order:0"`
	Pagination     *common.PaginationParams `query:"pagination" extensions:"x-order:1"`
}

type NftTxsRequest struct {
	CollectionAddr string                   `param:"collection_addr" extensions:"x-order:0"`
	TokenId        string                   `param:"token_id" extensions:"x-order:1"`
	Pagination     *common.PaginationParams `query:"pagination" extensions:"x-order:2"`
}

// Response

type Nft struct {
	CollectionAddr string `json:"collection_address" extensions:"x-order:0"`
	ObjectAddr     string `json:"object_addr,omitempty" extensions:"x-order:1"` // only used in Move
	TokenId        string `json:"nft_token_id" extensions:"x-order:2"`
	Owner          string `json:"owner" extensions:"x-order:3"`
	Uri            string `json:"uri" extensions:"x-order:4"`
}

type NftsResponse struct {
	Tokens     []Nft                `json:"tokens" extensions:"x-order:0"`
	Pagination *common.PageResponse `json:"pagination" extensions:"x-order:1"`
}

func ToResponseNft(nft *types.CollectedNft) *Nft {
	return &Nft{
		CollectionAddr: nft.CollectionAddr,
		ObjectAddr:     nft.Addr, // only used in Move
		TokenId:        nft.TokenId,
		Owner:          nft.Owner,
		Uri:            nft.Uri,
	}
}

func BatchToResponseNfts(nfts []types.CollectedNft) []Nft {
	nftResponses := make([]Nft, 0, len(nfts))
	for _, nft := range nfts {
		nftResponses = append(nftResponses, *ToResponseNft(&nft))
	}
	return nftResponses
}
