package nft

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

// Collection
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
	Height           int64            `json:"height" extensions:"x-order:2"`
	Timestamp        time.Time        `json:"timestamp" extensions:"x-order:3"`
}

type CollectionsResponse struct {
	Collections []Collection              `json:"collections" extensions:"x-order:0"`
	Pagination  common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

type CollectionResponse struct {
	Collection Collection `json:"collection"`
}

func ToCollectionResponse(col types.CollectedNftCollection, creatorAccount []byte) Collection {
	return Collection{
		Address: util.BytesToHexWithPrefix(col.Addr),
		CollectionDetail: CollectionDetail{
			Creator:    sdk.AccAddress(creatorAccount).String(),
			Name:       col.Name,
			OriginName: col.OriginName,
			Nfts: NftHandle{
				Handle: "",
				Length: col.NftCount,
			},
		},
		Height:    col.Height,
		Timestamp: col.Timestamp,
	}
}

func ToCollectionsResponse(cols []types.CollectedNftCollection, creatorAccounts map[int64][]byte) []Collection {
	collections := make([]Collection, 0, len(cols))
	for _, col := range cols {
		creatorAccount := creatorAccounts[col.CreatorId]
		collections = append(collections, ToCollectionResponse(col, creatorAccount))
	}
	return collections
}

// Nft
type NftDetails struct {
	TokenId string `json:"token_id" extensions:"x-order:2"`
	Uri     string `json:"uri" extensions:"x-order:3"`
}

type Nft struct {
	CollectionAddr       string     `json:"collection_addr" extensions:"x-order:0"`
	CollectionName       string     `json:"collection_name" extensions:"x-order:1"`
	CollectionOriginName string     `json:"collection_origin_name" extensions:"x-order:2"`
	ObjectAddr           string     `json:"object_addr" extensions:"x-order:3"` // only used in Move
	Owner                string     `json:"owner" extensions:"x-order:4"`
	Nft                  NftDetails `json:"nft" extensions:"x-order:5"`
	Height               int64      `json:"height" extensions:"x-order:6"`
	Timestamp            time.Time  `json:"timestamp" extensions:"x-order:7"`
}

type NftsResponse struct {
	Tokens     []Nft                     `json:"tokens" extensions:"x-order:0"`
	Pagination common.PaginationResponse `json:"pagination" extensions:"x-order:1"`
}

func ToNftResponse(name, originName string, nft types.CollectedNft, ownerAccount []byte) Nft {
	return Nft{
		CollectionAddr:       util.BytesToHexWithPrefix(nft.CollectionAddr),
		CollectionName:       name,
		CollectionOriginName: originName,
		ObjectAddr:           util.BytesToHexWithPrefix(nft.Addr), // only used in Move
		Owner:                sdk.AccAddress(ownerAccount).String(),
		Nft: NftDetails{
			TokenId: nft.TokenId,
			Uri:     nft.Uri,
		},
		Height:    nft.Height,
		Timestamp: nft.Timestamp,
	}
}

func ToNftsResponse(db *orm.Database, nfts []types.CollectedNft, ownerAccounts map[int64][]byte) ([]Nft, error) {
	nftResponses := make([]Nft, 0, len(nfts))
	for _, nft := range nfts {
		// get collection names and origin names
		collection, err := getCollectionByAddr(db, util.BytesToHexWithPrefix(nft.CollectionAddr))
		if err != nil {
			return nil, err
		}
		ownerAccount := ownerAccounts[nft.OwnerId]
		nftResponses = append(nftResponses, ToNftResponse(collection.Name, collection.OriginName, nft, ownerAccount))
	}
	return nftResponses, nil
}
