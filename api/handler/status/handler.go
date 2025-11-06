package status

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/cache"
	"github.com/initia-labs/rollytics/api/handler/common"
)

type StatusHandler struct {
	*common.BaseHandler
}

var _ common.HandlerRegistrar = (*StatusHandler)(nil)

func NewStatusHandler(base *common.BaseHandler) *StatusHandler {
	return &StatusHandler{BaseHandler: base}
}

func (h *StatusHandler) Register(router fiber.Router) {
	status := router.Group("/status")

	status.Get("/", cache.WithExpiration(time.Second), h.GetStatus)
}
