package cmd

import (
	"context"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.GetConfig()
			if err != nil {
				return err
			}

			logger := log.NewLogger(cfg)

			// Initialize the request limiter
			util.InitUtil(cfg)

			// Initialize dictionary caches
			util.InitializeCaches(cfg.GetCacheConfig())

			// Initialize database
			db, err := orm.OpenDB(cfg.GetDBConfig(), logger)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			// Initialize metrics
			metrics.Init()
			metricsServer := metrics.NewServer(cfg, logger)

			// Start metrics server in background
			go func() {
				if err := metricsServer.Start(); err != nil {
					logger.Error("metrics server failed", "error", err)
				}
			}()

			if sentryCfg := cfg.GetSentryConfig(); sentryCfg != nil {
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
				defer sentry.Flush(2 * time.Second)
			}

			// Start DB migration (check status and conditionally handle last migration)
			migrateResult, err := db.MigrateWithLastCheck()
			if err != nil {
				return err
			}

			// If last migration has atlas:txmode none, run it concurrently with indexer
			// Otherwise, run it first and then start indexer
			if migrateResult.LastMigrationHasTxModeNone {
				// Run last migration concurrently with indexer
				go func() {
					logger.Info("running last migration concurrently with indexer")
					if err := migrateResult.RunLastMigration(); err != nil {
						logger.Error("last migration failed", "error", err)
						sentry.CaptureException(err)
					} else {
						logger.Info("last migration completed successfully")
					}
					logger.Info("last migration completed successfully concurrently with indexer")
				}()
			} else {
				logger.Info("running migration sequentially with indexer")
				// Run last migration first, then start indexer
				if err := migrateResult.RunLastMigration(); err != nil {
					return err
				}
				logger.Info("last migration completed successfully sequentially with indexer")
			}

			// Apply patch
			if err := patcher.Patch(cfg, db, logger); err != nil {
				return err
			}

			// Initialize API server
			indexerAPI := indexerapi.New(cfg, logger, db)

			// Start API server in background
			go func() {
				if err := indexerAPI.Start(); err != nil {
					logger.Error("indexer API server failed", "error", err)
				}
			}()

			// Setup graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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

			// Start DB stats collection
			metrics.StartDBStatsUpdater(db, logger)
			defer metrics.StopDBStatsUpdater()

			idxer := indexer.New(cfg, logger, db)
			return idxer.Run(ctx)
		},
	}

	return cmd
}
