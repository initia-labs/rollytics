package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/config"
	"github.com/initia-labs/rollytics/api/handler/block"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/api/handler/nft"
	"github.com/initia-labs/rollytics/api/handler/tx"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

func Register(router fiber.Router, db *orm.Database, cfg *config.Config, logger *slog.Logger) {
	h := common.NewHandler(db, cfg, logger)

	registerBlockRoutes(router, h)
	registerTxRoutes(router, h)
	registerNftRoutes(router, h)
}

func registerBlockRoutes(router fiber.Router, h *common.Handler) {
	blockHandler := &block.BlockHandler{Handler: h}
	blocks := router.Group("/block/v1")

	blocks.Get("/blocks", blockHandler.GetBlocks)
	blocks.Get("/blocks/:height", blockHandler.GetBlockByHeight)
	blocks.Get("/avg_blocktime", blockHandler.GetAvgBlockTime)
}

func registerTxRoutes(router fiber.Router, h *common.Handler) {
	txHandler := &tx.TxHandler{Handler: h}
	txs := router.Group("/tx/v1")

	// Regular transaction routes
	txs.Get("/txs", txHandler.GetTxs)
	txs.Get("/txs/by_account/:account", txHandler.GetTxsByAccount)
	txs.Get("/txs/by_height/:height", txHandler.GetTxsByHeight)
	txs.Get("/txs/count", txHandler.GetTxsCount)
	txs.Get("/txs/:tx_hash", txHandler.GetTxByHash)

	// EVM Transaction routes (conditional)
	evmTxs := txs.Group("/evm-txs")
	if h.GetChainConfig().VmType == types.EVM {
		evmTxs.Get("", txHandler.GetEvmTxs)
		evmTxs.Get("/by_account/:account", txHandler.GetEvmTxsByAccount)
		evmTxs.Get("/by_height/:height", txHandler.GetEvmTxsByHeight)
		evmTxs.Get("/count", txHandler.GetEvmTxsCount)
		evmTxs.Get("/:tx_hash", txHandler.GetEvmTxByHash)
	} else {
		// non implementation for non-EVM chains
		evmTxs.Get("", notImplemented)
		evmTxs.Get("/by_account/:account", notImplemented)
		evmTxs.Get("/by_height/:height", notImplemented)
		evmTxs.Get("/count", notImplemented)
		evmTxs.Get("/:tx_hash", notImplemented)
	}
}

func registerNftRoutes(router fiber.Router, h *common.Handler) {
	nftHandler := &nft.NftHandler{Handler: h}
	nfts := router.Group("/nft/v1")

	// Collections routes
	collections := nfts.Group("/collections")
	collections.Get("/", nftHandler.GetCollections)
	collections.Get("/by_account/:account", nftHandler.GetCollectionsByAccount)
	collections.Get("/by_name/:name", nftHandler.GetCollectionsByName)
	collections.Get("/:collection_addr", nftHandler.GetCollectionByCollection)

	// Tokens(NFT) routes
	tokens := nfts.Group("/tokens")
	tokens.Get("/by_account/:account", nftHandler.GetTokensByAccount)
	tokens.Get("/by_collection/:collection_addr", nftHandler.GetTokensByCollection)

	// NFT transaction routes
	tokens.Get("/txs/:collection_addr/:token_id", nftHandler.GetNftTxs)
}

func notImplemented(c *fiber.Ctx) error {
	return fiber.NewError(fiber.StatusNotImplemented, "Not Implemented")
}
