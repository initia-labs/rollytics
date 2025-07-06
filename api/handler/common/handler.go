package common

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
	"github.com/initia-labs/rollytics/util"
)

type HandlerRegistrar interface {
	Register(router fiber.Router)
}

type BaseHandler struct {
	db     *orm.Database
	cfg    *config.Config
	logger *slog.Logger
}

func NewBaseHandler(db *orm.Database, cfg *config.Config, logger *slog.Logger) *BaseHandler {
	return &BaseHandler{
		db:     db,
		cfg:    cfg,
		logger: logger,
	}
}

func (h *BaseHandler) GetDatabase() *orm.Database { return h.db }
func (h *BaseHandler) GetConfig() *config.Config  { return h.cfg }
func (h *BaseHandler) GetLogger() *slog.Logger    { return h.logger }
func (h *BaseHandler) GetChainConfig() *config.ChainConfig {
	return h.cfg.GetChainConfig()
}
func (h *BaseHandler) GetChainId() string {
	return h.cfg.GetChainId()
}
func (h *BaseHandler) GetVmType() types.VMType {
	return h.cfg.GetChainConfig().VmType
}
func (h *BaseHandler) GetMsgTypeIds(msgs []string) ([]int64, error) {
	return util.GetOrCreateMsgTypeIds(h.db.DB, msgs, false)
}
