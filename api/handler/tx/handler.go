package tx

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

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
	txs := router.Group("/tx/v1")

	txs.Get("/txs", h.GetTxs)
	txs.Get("/txs/by_account/:account", h.GetTxsByAccount)
	txs.Get("/txs/by_height/:height", h.GetTxsByHeight)
	txs.Get("/txs/:tx_hash", h.GetTxByHash)

	evmTxs := txs.Group("/evm-txs")
	if h.GetChainConfig().VmType == types.EVM {
		evmTxs.Get("", h.GetEvmTxs)
		evmTxs.Get("/by_account/:account", h.GetEvmTxsByAccount)
		evmTxs.Get("/by_height/:height", h.GetEvmTxsByHeight)
		evmTxs.Get("/:tx_hash", h.GetEvmTxByHash)
	} else {
		evmTxs.All("/*", h.NotFound)
	}
}

func (h *TxHandler) GetLogger() *slog.Logger {
	return h.BaseHandler.GetLogger().With("handler", "tx")
}
