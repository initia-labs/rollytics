package main

import (
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/orm"
	"github.com/spf13/cobra"
)

func indexerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "indexer",
		Short: "run rollytics indexer",
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
