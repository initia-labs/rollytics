package cmd

import (
	"github.com/initia-labs/rollytics/api"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/orm"
	"github.com/spf13/cobra"
)

func apiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "api",
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
