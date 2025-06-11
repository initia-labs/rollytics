package common

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/initia-labs/rollytics/api/config"
	"github.com/initia-labs/rollytics/orm"
)

type HandlerRegistrar interface {
	Register(router fiber.Router)
}

type BaseHandler struct {
	database *orm.Database
	config   *config.Config
	logger   *slog.Logger
}

func NewBaseHandler(db *orm.Database, cfg *config.Config, logger *slog.Logger) *BaseHandler {
	return &BaseHandler{
		database: db,
		config:   cfg,
		logger:   logger,
	}
}

func (h *BaseHandler) GetDatabase() *orm.Database { return h.database }
func (h *BaseHandler) GetConfig() *config.Config  { return h.config }
func (h *BaseHandler) GetLogger() *slog.Logger    { return h.logger }
func (h *BaseHandler) GetChainConfig() *config.ChainConfig {
	return h.config.GetChainConfig()
}
