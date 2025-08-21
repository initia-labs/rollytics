package patcher

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/initia-labs/rollytics/config"
	"github.com/initia-labs/rollytics/orm"
	"github.com/initia-labs/rollytics/patcher/v1_0_2"
	"github.com/initia-labs/rollytics/types"
)

type PatchHandler struct {
	Version string
	Func    func(*gorm.DB, *config.Config) error
}

var patches = []PatchHandler{
	// should be ordered by version
	{"v1.0.2", v1_0_2.Patch},
}

func Patch(cfg *config.Config, db *orm.Database, logger *slog.Logger) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, patch := range patches {
			applied, err := patch.isApplied(tx)
			if err != nil {
				return fmt.Errorf("failed to check patch %s: %w", patch.Version, err)
			}

			if applied {
				logger.Info("Patch already applied, skipping", slog.String("version", patch.Version))
				continue
			}

			if err := patch.apply(tx, cfg); err != nil {
				return fmt.Errorf("failed to apply patch %s: %w", patch.Version, err)
			}
			logger.Info("Patch applied successfully", slog.String("version", patch.Version))
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	return err
}

func (p *PatchHandler) isApplied(db *gorm.DB) (bool, error) {
	var count int64
	err := db.Model(&types.CollectedPatch{}).Where("version = ?", p.Version).Count(&count).Error
	return count > 0, err
}

func (p *PatchHandler) apply(db *gorm.DB, cfg *config.Config) error {
	if err := p.Func(db, cfg); err != nil {
		return err
	}

	patch := &types.CollectedPatch{
		Version: p.Version,
		Applied: time.Now(),
	}

	return db.Create(patch).Error
}
