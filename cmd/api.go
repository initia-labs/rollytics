package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/initia-labs/rollytics/api"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/metrics"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/util/cache"
	"github.com/initia-labs/rollytics/util/querier"
)

func apiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Run rollytics API server",
		Long: `
Run the rollytics API server.

This command starts the HTTP API service for rollytics, providing endpoints for blockchain analytics and data access.

You can configure database, chain, logging, and server options via environment variables.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.GetConfig()
			if err != nil {
				return err
			}

			logger := log.NewLogger(cfg)

			// Initialize the request limiter
			// TODO: revisit
			querier.InitUtil(cfg)

			// Initialize dictionary caches
			cache.InitializeCaches(cfg.GetCacheConfig())

			// Initialize metrics
			metrics.Init()
			metricsServer := metrics.NewServer(cfg, logger)

			// Start metrics server in background
			go func() {
				if err := metricsServer.Start(); err != nil {
					logger.Error("metrics server failed", "error", err)
				}
			}()

			db, err := orm.OpenDB(cfg.GetDBConfig(), logger)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			// Start DB stats collection
			metrics.StartDBStatsUpdater(db, logger)

			server := api.New(cfg, logger, db)

			// graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigChan
				logger.Info("shutting down API server...")

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := metricsServer.Shutdown(ctx); err != nil {
					logger.Error("failed to shutdown metrics server", slog.String("error", err.Error()))
				}

				if err := server.Shutdown(); err != nil {
					logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
					os.Exit(1)
				}
				os.Exit(0)
			}()

			return server.Start()
		},
	}

	return cmd
}
