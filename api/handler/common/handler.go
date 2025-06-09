package common

import (
	"log/slog"

	"github.com/initia-labs/rollytics/api/config"
	"github.com/initia-labs/rollytics/orm"
)

type Handler struct {
	*orm.Database
	*config.Config
	*slog.Logger
}

func NewHandler(db *orm.Database, cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		Database: db,
		Config:   cfg,
		Logger:   logger,
	}
}
