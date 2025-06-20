package main

import (
	"github.com/initia-labs/rollytics/api"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/orm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func apiCmd() *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Use:   "api",
		Short: "run rollytics API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set LOG_FORMAT from the persistent flag before loading config
			viper.Set("LOG_FORMAT", LogFormat)

			if port != "" {
				viper.Set("LISTEN_ADDR", port)
			}

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

	cmd.Flags().StringVar(&port, "port", "8080", "Port to listen on (default 8080)")

	return cmd
}
