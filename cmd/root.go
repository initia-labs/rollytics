package cmd

import (
	"github.com/spf13/cobra"
)

var (
	Version    = "dev"
	CommitHash = "unknown"
	LogFormat  = "plain"
	LogLevel   = "warn"
)

func versionCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			if verbose {
				cmd.Printf("Version: %s\nCommit: %s\n", Version, CommitHash)
			} else {
				cmd.Println(Version)
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

	cmd.PersistentFlags().StringVar(&LogFormat, "log_format", "plain", "Log format: plain (default) or json")
	cmd.PersistentFlags().StringVar(&LogLevel, "log_level", "warn", "Log level: debug, info, warn (default), error")

	cmd.AddCommand(versionCmd())
	cmd.AddCommand(indexerCmd())
	cmd.AddCommand(apiCmd())
	cmd.AddCommand(migrateCmd())

	return cmd
}
