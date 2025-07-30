package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/api/handler/block"
	"github.com/initia-labs/rollytics/api/handler/common"
	"github.com/initia-labs/rollytics/api/handler/nft"
	"github.com/initia-labs/rollytics/api/handler/status"
	"github.com/initia-labs/rollytics/api/handler/tx"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
)

func Register(router fiber.Router, db *orm.Database, cfg *config.Config, logger *slog.Logger) {
	base := common.NewBaseHandler(db, cfg, logger)
	handlers := []common.HandlerRegistrar{
		status.NewStatusHandler(base),
		block.NewBlockHandler(base),
		tx.NewTxHandler(base),
		nft.NewNftHandler(base),
	}

	for _, handler := range handlers {
		handler.Register(router)
	}
}
