package internaltx

import (
	"log/slog"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
)

type InternalTxIndexer struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *InternalTxIndexer {
	return &InternalTxIndexer{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}

func (i *InternalTxIndexer) Run() error {
	return nil
}
