package extension

import (
	"log/slog"

	"golang.org/x/sync/errgroup"

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
	g          errgroup.Group
}

func New(cfg *config.Config, logger *slog.Logger, db *orm.Database) *ExtensionManager {
	var extensions []types.Extension
	// Internal Transaction
	if itxIndexer := internaltx.New(cfg, logger, db); itxIndexer != nil {
		extensions = append(extensions, itxIndexer)
	}
	return &ExtensionManager{
		cfg:        cfg,
		logger:     logger,
		db:         db,
		extensions: extensions,
	}
}

func (m *ExtensionManager) Run() {
	for _, extension := range m.extensions {
		ext := extension
		m.g.Go(func() error {
			m.logger.Info("Starting extension", slog.String("name", ext.Name()))
			if err := ext.Run(); err != nil {
				m.logger.Error("Extension error",
					slog.String("name", ext.Name()),
					slog.Any("error", err))

				return err
			}

			return nil
		})
	}

	if err := m.g.Wait(); err != nil {
		panic(err)
	}
}
