package extension

import (
	"log/slog"
	"sync"

	"github.com/initia-labs/rollytics/config"
	internaltx "github.com/initia-labs/rollytics/indexer/extension/internaltx"
	"github.com/initia-labs/rollytics/indexer/extension/types"
	"github.com/initia-labs/rollytics/orm"
)

type ExtensionManager struct {
	cfg        *config.Config
	logger     *slog.Logger
	db         *orm.Database
	extensions []types.Extension
	wg         sync.WaitGroup
}

func NewManager(cfg *config.Config, logger *slog.Logger, db *orm.Database) *ExtensionManager {
	return &ExtensionManager{
		cfg:    cfg,
		logger: logger,
		db:     db,
	}
}

func (m *ExtensionManager) RegisterExtensions() {
	// Internal Transaction
	if itxIndexer := internaltx.New(m.cfg, m.logger, m.db); itxIndexer != nil {
		m.extensions = append(m.extensions, itxIndexer)
	}

}

func (m *ExtensionManager) Run() error {
	m.RegisterExtensions()

	if len(m.extensions) == 0 {
		m.logger.Info("No extensions registered")
		return nil
	}

	for _, ext := range m.extensions {
		m.wg.Add(1)
		go func(extension types.Extension) {
			defer m.wg.Done()

			m.logger.Info("Starting extension", slog.String("name", extension.Name()))
			if err := extension.Run(); err != nil {
				m.logger.Error("Extension error",
					slog.String("name", extension.Name()),
					slog.Any("error", err))
			}
		}(ext)
	}

	m.wg.Wait()
	return nil
}
