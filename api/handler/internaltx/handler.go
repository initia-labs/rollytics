package internaltx

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/types"
)

type InternalTxHandler struct {
	*common.BaseHandler
}

var _ common.HandlerRegistrar = (*InternalTxHandler)(nil)

func NewInternalTxHandler(base *common.BaseHandler) *InternalTxHandler {
	return &InternalTxHandler{BaseHandler: base}
}

func (h *InternalTxHandler) Register(router fiber.Router) {
	txs := router.Group("indexer/tx/v1/evm-internal-txs")

	if h.GetChainConfig().VmType == types.EVM {
		txs.Get("", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxs)
		txs.Get("/by_height/:height", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxsByHeight)
		txs.Get("/:tx_hash", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetEvmInternalTxByHash)
		txs.Get("/by_account/:account", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxsByAccount)
	} else {
		h.GetLogger().Info("VM type is not EVM, registering NotFound handler")
		txs.All("/*", h.NotFound)
	}
}
