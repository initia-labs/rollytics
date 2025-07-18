package block

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"

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
	blocks := router.Group("indexer/block/v1")

	blocks.Get("/blocks", cache.New(cache.Config{Expiration: time.Second}), h.GetBlocks)
	blocks.Get("/blocks/:height", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetBlockByHeight)
	blocks.Get("/avg_blocktime", cache.New(cache.Config{Expiration: 10 * time.Second}), h.GetAvgBlockTime)
}
