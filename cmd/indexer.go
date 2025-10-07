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

			if err := db.Migrate(); err != nil {
				return err
			}

			// Apply patch
			if err := patcher.Patch(cfg, db, logger); err != nil {
				return err
			}

			// Start DB stats collection
			metrics.StartDBStatsUpdater(db, logger)
			defer metrics.StopDBStatsUpdater()

			if sentryCfg := cfg.GetSentryConfig(); sentryCfg != nil {
				sentryClientOptions := sentry.ClientOptions{
					Dsn:                sentryCfg.DSN,
					ServerName:         cfg.GetChainConfig().ChainId + "-rollytics-indexer",
					EnableTracing:      true,
					ProfilesSampleRate: sentryCfg.ProfilesSampleRate,
					SampleRate:         sentryCfg.SampleRate,
					TracesSampleRate:   sentryCfg.TracesSampleRate,
					Tags: map[string]string{
						"chain":     cfg.GetChainConfig().ChainId,
						"component": "rollytics-indexer",
					},
					Environment: sentryCfg.Environment,
				}
				if err := sentry.Init(sentryClientOptions); err != nil {
					return err
				}
				logger.Info("Sentry initialized")
				defer sentry.Flush(2 * time.Second)
			}

			idxer := indexer.New(cfg, logger, db)
			return idxer.Run(ctx)
		},
	}

	return cmd
}
