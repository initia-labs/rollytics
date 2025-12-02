package v1_0_15

import (
	"log/slog"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
)

func Patch(tx *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("deleting all evm-ret cleanup status data")

	// Delete all rows from evm_ret_cleanup_status table
	if err := tx.Exec("DELETE FROM evm_ret_cleanup_status").Error; err != nil {
		logger.Error("failed to delete evm-ret cleanup status data", slog.Any("error", err))
		return err
	}

	logger.Info("successfully deleted all evm-ret cleanup status data")
	return nil
}
