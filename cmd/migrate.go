package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/initia-labs/rollytics/config"
)

func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Generate a new database migration file",
		Long: `
Generate a new database migration file.

This command generates a new database migration file using GORM and Atlas.

You can configure database options via environment variables.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.GetConfig()
			if err != nil {
				return err
			}
			dsn := cfg.GetDBConfig().DSN
			migrationDir := fmt.Sprintf("file://%s", cfg.GetDBConfig().MigrationDir)

			// #nosec G204
			rawCmd := exec.CommandContext(context.Background(), "atlas", "migrate", "diff",
				"migration",
				"--env", "gorm",
				"--dev-url", dsn,
				"--dir", migrationDir,
			)
			rawCmd.Stdout = os.Stdout
			rawCmd.Stderr = os.Stderr

			return rawCmd.Run()
		},
	}

	return cmd
}
