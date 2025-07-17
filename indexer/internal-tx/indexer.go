package internal_tx

import (
	"fmt"
	"log/slog"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/types"
)

// Indexer is responsible for collecting and indexing internal transactions.
type Indexer struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *orm.Database
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *Indexer {
	return &Indexer{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}

func (i *Indexer) Run(heightChan <-chan int64) error {
	if i.cfg.GetVmType() != types.EVM && i.cfg.InternalTxEnabled() {
		return fmt.Errorf("unsupported, vm type: %v, internal tx enabled: %v", i.cfg.GetVmType(), i.cfg.InternalTxEnabled())
	}
	i.collect(heightChan)
	return nil
}
