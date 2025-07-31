package tx

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"

	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

type TxHandler struct {
	*common.BaseHandler
}

var _ common.HandlerRegistrar = (*TxHandler)(nil)

func NewTxHandler(base *common.BaseHandler) *TxHandler {
	return &TxHandler{BaseHandler: base}
}

func (h *TxHandler) Register(router fiber.Router) {
	txs := router.Group("indexer/tx/v1")

	txs.Get("/txs", cache.New(cache.Config{Expiration: time.Second}), h.GetTxs)
	txs.Get("/txs/by_account/:account", cache.New(cache.Config{Expiration: time.Second}), h.GetTxsByAccount)
	txs.Get("/txs/by_height/:height", cache.New(cache.Config{Expiration: time.Second}), h.GetTxsByHeight)
	txs.Get("/txs/:tx_hash", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetTxByHash)

	evmTxs := txs.Group("/evm-txs")
	if h.GetChainConfig().VmType == types.EVM {
		evmTxs.Get("", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmTxs)
		evmTxs.Get("/by_account/:account", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmTxsByAccount)
		evmTxs.Get("/by_height/:height", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmTxsByHeight)
		evmTxs.Get("/:tx_hash", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetEvmTxByHash)
	} else {
		evmTxs.All("/*", h.NotFound)
	}

	itxs := txs.Group("/evm-internal-txs")
	if h.GetChainConfig().VmType == types.EVM && h.GetConfig().GetInternalTxConfig().Enabled {
		itxs.Get("", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxs)
		itxs.Get("/by_height/:height", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxsByHeight)
		itxs.Get("/:tx_hash", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetEvmInternalTxsByHash)
		itxs.Get("/by_account/:account", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxsByAccount)
	} else {
		itxs.All("/*", h.NotFound)
	}
}
