package cmd

import (
	"github.com/initia-labs/rollytics/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func versionCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			if verbose {
				cmd.Printf("Version: %s\nCommit: %s\n", config.Version, config.CommitHash)
			} else {
				cmd.Println(config.Version)
			}
		},
	}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed version info")
	return cmd
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollytics [flags] [command]",
		Short: "rollytics - Minitia analytics and indexing tool",
		Long: `
rollytics is a Minitia analytics and indexing tool that provides
comprehensive data collection and API services for blockchain networks.`,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	cmd.PersistentFlags().String("log_format", "plain", "Log format: plain (default) or json")
	cmd.PersistentFlags().String("log_level", "warn", "Log level: debug, info, warn (default), error")
	_ = viper.BindPFlag("LOG_FORMAT", cmd.PersistentFlags().Lookup("log_format"))
	_ = viper.BindPFlag("LOG_LEVEL", cmd.PersistentFlags().Lookup("log_level"))

	cmd.AddCommand(versionCmd())
	cmd.AddCommand(indexerCmd())
	cmd.AddCommand(apiCmd())
	cmd.AddCommand(migrateCmd())

	return cmd
}
