package cmd

import (
	"github.com/spf13/cobra"

	"github.com/initia-labs/rollytics/api"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/orm"
)

func apiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "run rollytics API server",
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
			db, err := orm.OpenDB(cfg.GetDBConfig(), logger)
			if err != nil {
				return err
			}

			server := api.New(cfg, logger, db)
			return server.Start()
		},
	}

	return cmd
}
