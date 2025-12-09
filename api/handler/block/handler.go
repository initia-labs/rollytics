package block

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util/common-handler/common"
	"github.com/initia-labs/rollytics/util/querier"
)

var _ common.HandlerRegistrar = (*BlockHandler)(nil)

type BlockHandler struct {
	*common.BaseHandler
	querier *querier.Querier
}

func NewBlockHandler(base *common.BaseHandler, cfg *config.Config) *BlockHandler {
	return &BlockHandler{BaseHandler: base, querier: querier.NewQuerier(cfg.GetChainConfig())}
}

func (h *BlockHandler) Register(router fiber.Router) {
	// TODO: recheck init cache
	// initValidatorCache(h.GetConfig())
	blocks := router.Group("indexer/block/v1")

	blocks.Get("/blocks", cache.WithExpiration(time.Second), h.GetBlocks)
	blocks.Get("/blocks/:height", cache.WithExpiration(10*time.Second), h.GetBlockByHeight)
	blocks.Get("/avg_blocktime", cache.WithExpiration(10*time.Second), h.GetAvgBlockTime)
}
