package extension

import (
	"context"
	"errors"
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

func (m *ExtensionManager) Run(ctx context.Context) {
	if len(m.extensions) == 0 {
		m.logger.Info("No extensions configured")
		return
	}

	// Use context-aware errgroup for coordinated shutdown
	g, gCtx := errgroup.WithContext(ctx)

	for _, extension := range m.extensions {
		ext := extension // Capture for goroutine
		g.Go(func() error {
			m.logger.Info("Starting extension", slog.String("name", ext.Name()))

			// All extensions now implement Run(ctx)
			err := ext.Run(gCtx)

			if err != nil {
				if errors.Is(err, context.Canceled) {
					m.logger.Info("Extension stopped",
						slog.String("name", ext.Name()),
						slog.String("reason", "context cancelled"))
					return nil // Context cancellation is expected during shutdown
				}

				m.logger.Error("Extension error",
					slog.String("name", ext.Name()),
					slog.Any("error", err))
				return err
			}

			m.logger.Info("Extension completed", slog.String("name", ext.Name()))
			return nil
		})
	}

	// Wait for all extensions to complete
	if err := g.Wait(); err != nil {
		if !errors.Is(err, context.Canceled) {
			m.logger.Error("Extension manager fatal error", slog.Any("error", err))
			// Only panic for actual errors, not context cancellation
			panic(err)
		}
	}

	m.logger.Info("Extension manager shutdown complete")
}
