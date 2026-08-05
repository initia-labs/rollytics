package richlist

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util/common-handler/common"
	"github.com/initia-labs/rollytics/util/querier"
)

type RichListHandler struct {
	*common.BaseHandler
	cfg     *config.Config
	querier *querier.Querier
}

var _ common.HandlerRegistrar = (*RichListHandler)(nil)

func NewRichListHandler(base *common.BaseHandler, cfg *config.Config) *RichListHandler {
	return &RichListHandler{
		BaseHandler: base,
		cfg:         cfg,
		querier:     querier.NewQuerier(cfg.GetChainConfig()),
	}
}

func (h *RichListHandler) Register(router fiber.Router) {
	richlist := router.Group("indexer/richlist/v1")
	// Wildcard so denoms with '/' (move/..., ibc/...) work without requiring %2F.
	richlist.Get("/*", cache.WithExpiration(10*time.Second), h.GetTokenHolders)
}
