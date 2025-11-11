package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/util/common-handler/common"
	"github.com/initia-labs/rollytics/util/common-handler/status"
)

func Register(router fiber.Router, db *orm.Database, cfg *config.Config, logger *slog.Logger) {
	base := common.NewBaseHandler(db, cfg, logger)
	handlers := []common.HandlerRegistrar{
		status.NewStatusHandler(base),
	}

	for _, handler := range handlers {
		handler.Register(router)
	}
}
