package richlist

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/util/common-handler/common"
)

type RichListHandler struct {
	*common.BaseHandler
	cfg *config.Config
}

var _ common.HandlerRegistrar = (*RichListHandler)(nil)

func NewRichListHandler(base *common.BaseHandler, cfg *config.Config) *RichListHandler {
	return &RichListHandler{
		BaseHandler: base,
		cfg:         cfg,
	}
}

func (h *RichListHandler) Register(router fiber.Router) {
	richlist := router.Group("richlist/v1")
	richlist.Get("/:denom", cache.WithExpiration(10*time.Second), h.GetTokenHolders)
}
