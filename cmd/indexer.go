package cmd

import (
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/indexer"
	"github.com/initia-labs/rollytics/log"
	"github.com/spf13/cobra"
)

func indexerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "indexer",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, cfgErr := config.GetConfig()
			if cfgErr != nil {
				panic(cfgErr)
			}

			logger := log.NewLogger(cfg)
			idxer, err := indexer.New(cfg, logger)
			if err != nil {
				panic(err)
			}

			idxer.Run()
		},
	}

	return cmd
}
