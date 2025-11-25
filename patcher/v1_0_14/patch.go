package v1_0_14

import (
	"log/slog"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
)

func Patch(tx *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("deleting all richlist data")

	// Delete all rows from rich_list table
	if err := tx.Exec("DELETE FROM rich_list").Error; err != nil {
		logger.Error("failed to delete richlist data", slog.Any("error", err))
		return err
	}

	// Delete all rows from rich_list_status table
	if err := tx.Exec("DELETE FROM rich_list_status").Error; err != nil {
		logger.Error("failed to delete richlist status data", slog.Any("error", err))
		return err
	}

	logger.Info("successfully deleted all richlist data")
	return nil
}
