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
	// Use context-aware errgroup
	g, gCtx := errgroup.WithContext(ctx)

	for _, extension := range m.extensions {
		ext := extension
		g.Go(func() error {
			m.logger.Info("Starting extension", slog.String("name", ext.Name()))

			// Check if extension supports context-aware Run method
			if ctxAwareExt, ok := ext.(types.ContextAwareExtension); ok {
				if err := ctxAwareExt.RunWithContext(gCtx); err != nil {
					m.logger.Error("Extension error",
						slog.String("name", ext.Name()),
						slog.Any("error", err))
					return err
				}
			} else {
				// Fallback for extensions that don't support context yet
				// Run in a separate goroutine and listen for context cancellation
				done := make(chan error, 1)
				go func() {
					done <- ext.Run()
				}()

				select {
				case err := <-done:
					if err != nil {
						m.logger.Error("Extension error",
							slog.String("name", ext.Name()),
							slog.Any("error", err))
						return err
					}
				case <-gCtx.Done():
					m.logger.Info("Extension shutting down", slog.String("name", ext.Name()))
					return gCtx.Err()
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		m.logger.Error("Extension manager error", slog.Any("error", err))
		// Don't panic on context cancellation, it's expected during shutdown
		if !errors.Is(err, context.Canceled) {
			panic(err)
		}
	}
}
