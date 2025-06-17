package cmd

import (
	"github.com/initia-labs/rollytics/api"
	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/log"
	"github.com/spf13/cobra"
)

func apiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "api",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, cfgErr := config.GetConfig()
			if cfgErr != nil {
				panic(cfgErr)
			}

			logger := log.NewLogger(cfg)
			server, err := api.New(cfg, logger)
			if err != nil {
				panic(err)
			}

			server.Start()
		},
	}

	return cmd
}
