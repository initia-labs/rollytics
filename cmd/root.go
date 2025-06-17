package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "rollytics",
	}

	cmd.AddCommand(indexerCmd())
	cmd.AddCommand(apiCmd())

	return cmd
}
