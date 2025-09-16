package patcher

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/patcher/v1_0_11"
	"github.com/initia-labs/rollytics/patcher/v1_0_2"
	"github.com/initia-labs/rollytics/types"
)

type PatchHandler struct {
	Version string
	Func    func(*gorm.DB, *config.Config, *slog.Logger) error
}

var patches = []PatchHandler{
	// Patches must be ordered by version in ascending order (oldest to newest).
	// Each patch builds on the previous ones, so the order is critical.
	{"v1.0.2", v1_0_2.Patch},
	{"v1.0.11", v1_0_11.Patch},
}

// Patch applies data migration patches to fix or update existing data in the database.
// Similar to schema migrations, but for data corrections and updates.
// Each patch is applied only once and tracked in the upgrade_history table.
// Patches are applied in order and wrapped in a serializable transaction for consistency.
func Patch(cfg *config.Config, db *orm.Database, logger *slog.Logger) error {
	for _, ph := range patches {
		if err := db.Transaction(func(tx *gorm.DB) error {
			applied, err := ph.isApplied(tx)
			if err != nil {
				return fmt.Errorf("failed to check patch %s: %w", ph.Version, err)
			}
			if applied {
				logger.Info("Patch already applied, skipping", slog.String("version", ph.Version))
				return nil
			}
			if err := ph.apply(tx, cfg, logger); err != nil {
				return fmt.Errorf("failed to apply patch %s: %w", ph.Version, err)
			}
			logger.Info("Patch applied successfully", slog.String("version", ph.Version))
			return nil
		}, &sql.TxOptions{Isolation: sql.LevelSerializable}); err != nil {
			return err
		}
	}
	return nil
}

func (p *PatchHandler) isApplied(db *gorm.DB) (bool, error) {
	var count int64
	err := db.Model(&types.CollectedUpgradeHistory{}).Where("version = ?", p.Version).Count(&count).Error
	return count > 0, err
}

func (p *PatchHandler) apply(db *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	if err := p.Func(db, cfg, logger); err != nil {
		return err
	}

	patch := &types.CollectedUpgradeHistory{
		Version: p.Version,
		Applied: time.Now(),
	}

	return db.Create(patch).Error
}
