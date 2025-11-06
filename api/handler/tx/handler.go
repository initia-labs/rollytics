package tx

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util/common-handler/common"
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

	txs.Get("/txs", cache.WithExpiration(time.Second), h.GetTxs)
	txs.Get("/txs/by_account/:account", cache.WithExpiration(time.Second), h.GetTxsByAccount)
	txs.Get("/txs/by_height/:height", cache.WithExpiration(time.Second), h.GetTxsByHeight)
	txs.Get("/txs/:tx_hash", cache.WithExpiration(10*time.Second), h.GetTxByHash)

	evmTxs := txs.Group("/evm-txs")
	if h.GetChainConfig().VmType == types.EVM {
		evmTxs.Get("", cache.WithExpiration(time.Second), h.GetEvmTxs)
		evmTxs.Get("/by_account/:account", cache.WithExpiration(time.Second), h.GetEvmTxsByAccount)
		evmTxs.Get("/by_height/:height", cache.WithExpiration(time.Second), h.GetEvmTxsByHeight)
		evmTxs.Get("/:tx_hash", cache.WithExpiration(10*time.Second), h.GetEvmTxByHash)
	} else {
		evmTxs.All("/*", h.NotFound)
	}

	itxs := txs.Group("/evm-internal-txs")
	if h.GetChainConfig().VmType == types.EVM && h.GetConfig().GetInternalTxConfig().Enabled {
		itxs.Get("", cache.WithExpiration(time.Second), h.GetEvmInternalTxs)
		itxs.Get("/by_height/:height", cache.WithExpiration(time.Second), h.GetEvmInternalTxsByHeight)
		itxs.Get("/:tx_hash", cache.WithExpiration(10*time.Second), h.GetEvmInternalTxsByHash)
		itxs.Get("/by_account/:account", cache.WithExpiration(time.Second), h.GetEvmInternalTxsByAccount)
	} else {
		itxs.All("/*", h.NotFound)
	}
}
