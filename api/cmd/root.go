package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "rollytics-api",
	}

	cmd.AddCommand(ApiCmd())

	return cmd
}
