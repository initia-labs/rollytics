package curated

import (
	"fmt"
	"log/slog"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
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
	if i.cfg.GetVmType() != types.EVM {
		return fmt.Errorf("unsupported VM type: %v", i.cfg.GetVmType())
	}
	i.collect()
	return nil
}
