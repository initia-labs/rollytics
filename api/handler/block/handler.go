package block

import (
	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/common"
)

type BlockHandler struct {
	*common.BaseHandler
}

var _ common.HandlerRegistrar = (*BlockHandler)(nil)

func NewBlockHandler(base *common.BaseHandler) *BlockHandler {
	return &BlockHandler{BaseHandler: base}
}

func (h *BlockHandler) Register(router fiber.Router) {
	blocks := router.Group("/block/v1")

	blocks.Get("/blocks", h.GetBlocks)
	blocks.Get("/blocks/:height", h.GetBlockByHeight)
	blocks.Get("/avg_blocktime", h.GetAvgBlockTime)
}
