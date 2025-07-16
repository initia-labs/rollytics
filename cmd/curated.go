package cmd

import (
	"github.com/spf13/cobra"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/curated"
	"github.com/initia-labs/rollytics/log"
	"github.com/initia-labs/rollytics/orm"
)

func evmInternalTxCmd() *cobra.Command {
	cmd :=
		&cobra.Command{
			Use:   "curated",
			Short: "Run Curated Indexer for EVM Internal Transactions",
			Long:  "Start the Rollytics service to index internal transactions on EVM-compatible chains",
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
				defer db.Close()

				indexer := curated.New(cfg, logger, db)
				return indexer.Run()
			},
		}

	return cmd
}
