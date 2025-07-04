package nft

import (
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

// Request
// Colections
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

// Tokens
type TokensByAccountRequest struct {
	Account        string                   `param:"account" extensions:"x-order:0"`
	CollectionAddr string                   `param:"collection_addr" extensions:"x-order:1"`
	TokenId        string                   `param:"token_id" extensions:"x-order:2"`
	Pagination     *common.PaginationParams `query:"pagination" extensions:"x-order:3"`
}

type TokensByCollectionRequest struct {
	CollectionAddr string                   `param:"collection_addr" extensions:"x-order:0"`
	TokenId        string                   `param:"token_id" extensions:"x-order:1"`
	Pagination     *common.PaginationParams `query:"pagination" extensions:"x-order:2"`
}

// Txs
type NftTxsRequest struct {
	CollectionAddr string                   `param:"collection_addr" extensions:"x-order:0"`
	TokenId        string                   `param:"token_id" extensions:"x-order:1"`
	Pagination     *common.PaginationParams `query:"pagination" extensions:"x-order:2"`
}

// Response
// Collections

type NftHandle struct {
	Handle string `json:"handle" extensions:"x-order:0"`
	Length int64  `json:"length" extensions:"x-order:1"`
}

type CollectionDetail struct {
	Creator    string    `json:"creator" extensions:"x-order:0"`
	Name       string    `json:"name" extensions:"x-order:1"`
	OriginName string    `json:"origin_name" extensions:"x-order:2"`
	Nfts       NftHandle `json:"nfts" extensions:"x-order:4"`
}

type Collection struct {
	Address          string           `json:"object_addr" extensions:"x-order:0"`
	CollectionDetail CollectionDetail `json:"collection" extensions:"x-order:1"`
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
		Address: col.Addr,
		CollectionDetail: CollectionDetail{
			Creator:    col.Creator,
			Name:       col.Name,
			OriginName: col.OriginName,
			Nfts: NftHandle{
				Handle: "",
				Length: col.NftCount,
			},
		},
	}
}

func BatchToResponseCollections(cols []types.CollectedNftCollection) (collections []Collection) {
	for _, col := range cols {
		collections = append(collections, *ToResponseCollection(&col))
	}
	return collections
}

// Tokens
type NftDetails struct {
	TokenId string `json:"token_id" extensions:"x-order:2"`
	Uri     string `json:"uri" extensions:"x-order:3"`
}

type Nft struct {
	CollectionAddr       string     `json:"collection_addr" extensions:"x-order:0"`
	CollectionName       string     `json:"collection_name" extensions:"x-order:1"`
	CollectionOriginName string     `json:"collection_origin_name" extensions:"x-order:2"`
	ObjectAddr           string     `json:"object_addr,omitempty" extensions:"x-order:3"` // only used in Move
	Owner                string     `json:"owner_addr" extensions:"x-order:4"`
	Nft                  NftDetails `json:"nft" extensions:"x-order:5"`
}

type NftsResponse struct {
	Tokens     []Nft                `json:"tokens" extensions:"x-order:0"`
	Pagination *common.PageResponse `json:"pagination" extensions:"x-order:1"`
}

func ToResponseNft(name, originName string, nft *types.CollectedNft) *Nft {
	return &Nft{
		CollectionAddr:       nft.CollectionAddr,
		CollectionName:       name,
		CollectionOriginName: originName,
		ObjectAddr:           nft.Addr, // only used in Move
		Owner:                nft.Owner,
		Nft: NftDetails{
			TokenId: nft.TokenId,
			Uri:     nft.Uri,
		},
	}
}

func BatchToResponseNfts(db *orm.Database, nfts []types.CollectedNft) (nftResponses []Nft, err error) {
	for _, nft := range nfts {
		// get collection names and origin names
		collection, err := getCollection(db, nft.CollectionAddr)
		if err != nil {
			return nil, fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		nftResponses = append(nftResponses, *ToResponseNft(collection.Name, collection.OriginName, &nft))
	}
	return nftResponses, nil
}
