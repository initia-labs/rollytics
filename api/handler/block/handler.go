package block

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

type BlockHandler struct {
	*common.BaseHandler
}

var _ common.HandlerRegistrar = (*BlockHandler)(nil)

func NewBlockHandler(base *common.BaseHandler) *BlockHandler {
	return &BlockHandler{BaseHandler: base}
}

func (h *BlockHandler) Register(router fiber.Router) {
	initValidatorCache(h.GetConfig())
	blocks := router.Group("indexer/block/v1")

	blocks.Get("/blocks", cache.WithExpiration(time.Second), h.GetBlocks)
	blocks.Get("/blocks/:height", cache.WithExpiration(10*time.Second), h.GetBlockByHeight)
	blocks.Get("/avg_blocktime", cache.WithExpiration(10*time.Second), h.GetAvgBlockTime)
}
