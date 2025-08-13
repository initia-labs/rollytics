package common

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/metrics"
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

func (h *BaseHandler) GetAccountIds(accounts []string) ([]int64, error) {
	idMap, err := util.GetOrCreateAccountIds(h.db.DB, accounts, false)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, acc := range accounts {
		if id, ok := idMap[acc]; ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (h *BaseHandler) GetMsgTypeIds(msgs []string) ([]int64, error) {
	idMap, err := util.GetOrCreateMsgTypeIds(h.db.DB, msgs, false)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, msg := range msgs {
		if id, ok := idMap[msg]; ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (h *BaseHandler) GetNftIds(keys []util.NftKey) ([]int64, error) {
	idMap, err := util.GetOrCreateNftIds(h.db.DB, keys, false)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, key := range keys {
		if id, ok := idMap[key]; ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// TrackError tracks errors in handlers
func (h *BaseHandler) TrackError(errorType string) {
	metrics.TrackError("api", errorType)
}
