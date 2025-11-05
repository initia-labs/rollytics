package richlist

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/api/handler/common"
)

type RichListHandler struct {
	*common.BaseHandler
}

var _ common.HandlerRegistrar = (*RichListHandler)(nil)

func NewRichListHandler(base *common.BaseHandler) *RichListHandler {
	return &RichListHandler{BaseHandler: base}
}

func (h *RichListHandler) Register(router fiber.Router) {
	richlist := router.Group("richlist/v1")
	richlist.Get("/:denom", cache.WithExpiration(10*time.Second), h.GetTokenHolders)
}
