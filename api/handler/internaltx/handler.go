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

func NewTxHandler(base *common.BaseHandler) *InternalTxHandler {
	return &InternalTxHandler{BaseHandler: base}
}

func (h *InternalTxHandler) Register(router fiber.Router) {
	txs := router.Group("/tx/v1")

	if h.GetChainConfig().VmType == types.EVM {
		txs.Get("/internal-txs", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxs)
		txs.Get("/internal-txs/by_height/:height", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxsByHeight)
		txs.Get("/internal-txs/:tx_hash", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetEvmInternalTxByHash)
		txs.Get("/internal-txs/by_account/:account", cache.New(cache.Config{Expiration: time.Second}), h.GetEvmInternalTxsByAccount)
	} else {
		txs.All("/*", h.NotFound)
	}
}

