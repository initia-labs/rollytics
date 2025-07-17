package nft

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"

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
	nfts := router.Group("indexer/nft/v1")

	// Collections routes
	collections := nfts.Group("/collections")
	collections.Get("/", cache.New(cache.Config{Expiration: time.Second}), h.GetCollections)
	collections.Get("/by_account/:account", cache.New(cache.Config{Expiration: time.Second}), h.GetCollectionsByAccount)
	collections.Get("/by_name/:name", cache.New(cache.Config{Expiration: time.Second}), h.GetCollectionsByName)
	collections.Get("/:collection_addr", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetCollectionByCollectionAddr)

	// Tokens(NFT) routes
	tokens := nfts.Group("/tokens")
	tokens.Get("/by_account/:account", cache.New(cache.Config{Expiration: time.Second}), h.GetTokensByAccount)
	tokens.Get("/by_collection/:collection_addr", cache.New(cache.Config{Expiration: time.Second}), h.GetTokensByCollectionAddr)

	// NFT transaction routes
	txs := nfts.Group("/txs")
	txs.Get("/:collection_addr/:token_id", cache.New(cache.Config{Expiration: time.Second}), h.GetNftTxs)
}
