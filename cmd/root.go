package main

import (
	"github.com/spf13/cobra"
)

var (
	Version    = "dev"
	CommitHash = "unknown"
)

func versionCmd() *cobra.Command {
	var long bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			if long {
				cmd.Printf("Version: %s\nCommit: %s\n", Version, CommitHash)
			} else {
				cmd.Println(Version)
			}
		},
	}
	cmd.Flags().BoolVar(&long, "long", false, "Show detailed version info")
	return cmd
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollytics [flags] [command]",
		Short: "rollytics - Minitia analytics and indexing tool",
		Long: `rollytics is a Minitia analytics and indexing tool that provides
comprehensive data collection and API services for blockchain networks.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(versionCmd())
	cmd.AddCommand(indexerCmd())
	cmd.AddCommand(apiCmd())

	return cmd
}
