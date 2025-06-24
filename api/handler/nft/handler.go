package nft

import (
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
)

type NftHandler struct {
	*common.BaseHandler
}
 
var _ common.HandlerRegistrar = (*NftHandler)(nil)

func NewNftHandler(base *common.BaseHandler) *NftHandler {
	return &NftHandler{BaseHandler: base}
}

func (h *NftHandler) Register(router fiber.Router) {
	nfts := router.Group("/nft/v1")

	// Collections routes
	collections := nfts.Group("/collections")
	collections.Get("/", h.GetCollections)
	collections.Get("/by_account/:account", h.GetCollectionsByOwner)
	collections.Get("/by_name/:name", h.GetCollectionsByName)
	collections.Get("/:collection_addr", h.GetCollectionByCollectionAddr)

	// Tokens(NFT) routes
	tokens := nfts.Group("/tokens")
	tokens.Get("/by_account/:account", h.GetTokensByOwner)
	tokens.Get("/by_collection/:collection_addr", h.GetTokensByCollectionAddr)

	// NFT transaction routes
	txs := nfts.Group("/txs")
	txs.Get("/:collection_addr/:token_id", h.GetNftTxs)
}
