package cmd

import (
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/orm"
	"github.com/spf13/cobra"
)

func indexerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "indexer",
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

			if err := db.Migrate(); err != nil {
				return err
			}

			idxer := indexer.New(cfg, logger, db)
			return idxer.Run()
		},
	}

	return cmd
}
