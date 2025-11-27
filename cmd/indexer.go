package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer"
	indexerapi "github.com/initia-labs/rollytics/indexer/api"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/patcher"
	"github.com/initia-labs/rollytics/util"
)

func indexerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "indexer",
		Short: "Run rollytics indexer",
		Long: `
Run the rollytics indexer service.

This command starts the blockchain indexer, which collects and processes on-chain data for analytics and storage.

You can configure database, chain, logging, and indexer options via environment variables.`,
		RunE: runIndexer,
	}

	return cmd
}

func runIndexer(cmd *cobra.Command, args []string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	logger := log.NewLogger(cfg)
	initializeUtilities(cfg)

	db, err := initializeDatabase(cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	metricsServer, err := initializeMetrics(cfg, logger)
	if err != nil {
		return err
	}

	shouldFlushSentry, err := initializeSentryIfConfigured(cfg, logger)
	if err != nil {
		return err
	}
	if shouldFlushSentry {
		defer sentry.Flush(2 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := handleMigrations(ctx, db, logger); err != nil {
		return err
	}

	if err := patcher.Patch(cfg, db, logger); err != nil {
		return err
	}

	indexerAPI, err := initializeAPIServer(cfg, logger, db)
	if err != nil {
		return err
	}

	setupGracefulShutdown(ctx, cancel, logger, indexerAPI, metricsServer)

	metrics.StartDBStatsUpdater(db, logger)
	defer metrics.StopDBStatsUpdater()

	idxer := indexer.New(cfg, logger, db)
	return idxer.Run(ctx)
}

func initializeUtilities(cfg *config.Config) {
	util.InitUtil(cfg)
	util.InitializeCaches(cfg.GetCacheConfig())
}

func initializeDatabase(cfg *config.Config, logger *slog.Logger) (*orm.Database, error) {
	return orm.OpenDB(cfg.GetDBConfig(), logger)
}

func initializeMetrics(cfg *config.Config, logger *slog.Logger) (*metrics.MetricsServer, error) {
	metrics.Init()
	metricsServer := metrics.NewServer(cfg, logger)

	go func() {
		if err := metricsServer.Start(); err != nil {
			logger.Error("metrics server failed", "error", err)
		}
	}()

	return metricsServer, nil
}

func initializeSentryIfConfigured(cfg *config.Config, logger *slog.Logger) (bool, error) {
	sentryCfg := cfg.GetSentryConfig()
	if sentryCfg == nil {
		return false, nil
	}

	if err := initializeSentry(cfg, logger, sentryCfg); err != nil {
		return false, err
	}

	return true, nil
}

func handleMigrations(ctx context.Context, db *orm.Database, logger *slog.Logger) error {
	concurrentLastMigration, err := db.CheckLastMigrationConcurrency()
	if err != nil {
		return err
	}

	if concurrentLastMigration {
		go func() {
			// Check if context is already cancelled before starting migration
			select {
			case <-ctx.Done():
				logger.Info("shutdown requested, skipping migration")
				return
			default:
			}

			logger.Info("running last migration concurrently with indexer")

			// Run migration in a goroutine so we can monitor context cancellation
			migrationDone := make(chan error, 1)
			go func() {
				migrationDone <- db.Migrate()
			}()

			select {
			case <-ctx.Done():
				logger.Info("shutdown requested, migration may still be running")
				return
			case err := <-migrationDone:
				if err != nil {
					logger.Error("last migration failed", "error", err)
					sentry.CaptureException(err)
				} else {
					logger.Info("last migration completed successfully")
				}
			}
		}()
	}

	return nil
}

func initializeAPIServer(cfg *config.Config, logger *slog.Logger, db *orm.Database) (*indexerapi.Api, error) {
	indexerAPI := indexerapi.New(cfg, logger, db)

	go func() {
		if err := indexerAPI.Start(); err != nil {
			logger.Error("indexer API server failed", "error", err)
		}
	}()

	return indexerAPI, nil
}

func setupGracefulShutdown(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger, indexerAPI *indexerapi.Api, metricsServer *metrics.MetricsServer) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("shutting down indexer...")
		if err := indexerAPI.Shutdown(); err != nil {
			logger.Error("failed to shutdown indexer API server", "error", err)
		}
		if err := metricsServer.Shutdown(ctx); err != nil {
			logger.Error("failed to shutdown metrics server", "error", err)
		}
		cancel()
	}()
}

func initializeSentry(cfg *config.Config, logger *slog.Logger, sentryCfg *config.SentryConfig) error {
	sentryClientOptions := sentry.ClientOptions{
		Dsn:                sentryCfg.DSN,
		ServerName:         cfg.GetChainConfig().ChainId + "-rollytics-indexer",
		EnableTracing:      true,
		ProfilesSampleRate: sentryCfg.ProfilesSampleRate,
		SampleRate:         sentryCfg.SampleRate,
		TracesSampleRate:   sentryCfg.TracesSampleRate,
		Tags: map[string]string{
			"chain":       cfg.GetChainConfig().ChainId,
			"component":   "rollytics-indexer",
			"environment": sentryCfg.Environment,
		},
		Environment: sentryCfg.Environment,
	}
	if err := sentry.Init(sentryClientOptions); err != nil {
		return err
	}
	logger.Info("Sentry initialized", "chain", cfg.GetChainConfig().ChainId, "component", "rollytics-indexer", "environment", sentryCfg.Environment)
	return nil
}
