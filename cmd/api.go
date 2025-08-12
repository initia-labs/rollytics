package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/initia-labs/rollytics/api"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/util"
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
			util.InitLimiter(cfg)
			
			db, err := orm.OpenDB(cfg.GetDBConfig(), logger)
			if err != nil {
				return err
			}
			defer db.Close() //nolint:errcheck

			server := api.New(cfg, logger, db)

			// graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigChan
				logger.Info("shutting down API server...")
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
