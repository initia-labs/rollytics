package cmd

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer"
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

			// Initialize metrics
			metrics.Init()
			metricsServer := metrics.NewServer(cfg, logger)

			// Start metrics server in background
			go func() {
				if err := metricsServer.Start(); err != nil {
					logger.Error("metrics server failed", "error", err)
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
				if err := metricsServer.Shutdown(ctx); err != nil {
					logger.Error("failed to shutdown metrics server", "error", err)
				}
				cancel()
			}()

			db, err := orm.OpenDB(cfg.GetDBConfig(), logger)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			// Setup Sentry (doesn't depend on DB)
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

			// Migration must complete before indexer queries DB
			// Indexer's wait() for chain gives migration time, but we'll use a sync mechanism
			var migrationDone sync.WaitGroup
			migrationDone.Add(1)
			var migrationErr error

			// Create indexer before starting concurrent operations
			idxer := indexer.New(cfg, logger, db)

			// Ensure DB stats cleanup
			defer metrics.StopDBStatsUpdater()

			// Start migration and indexer concurrently
			g, gCtx := errgroup.WithContext(ctx)

			// Run migration
			g.Go(func() error {
				defer migrationDone.Done()
				migrationErr = db.Migrate()
				if migrationErr != nil {
					return migrationErr
				}

				// Apply patch after migration completes
				if err := patcher.Patch(cfg, db, logger); err != nil {
					return err
				}

				// Start DB stats collection after migration
				metrics.StartDBStatsUpdater(db, logger)
				return nil
			})

			// Run indexer - it will wait for chain first (giving migration time)
			// but we ensure migration completes before DB query
			g.Go(func() error {
				// Wait for migration to complete before indexer queries DB
				migrationDone.Wait()
				if migrationErr != nil {
					return migrationErr
				}

				return idxer.Run(gCtx)
			})

			// Wait for both to complete
			if err := g.Wait(); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}
