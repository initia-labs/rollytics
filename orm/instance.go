package orm

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"ariga.io/atlas-go-sdk/atlasexec"
	_ "ariga.io/atlas-provider-gorm/gormschema"
	sloggorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"github.com/initia-labs/rollytics/orm/config"
)

var (
	UpdateAllWhenConflict = clause.OnConflict{
		UpdateAll: true,
	}
	DoNothingWhenConflict = clause.OnConflict{
		DoNothing: true,
	}
)

type Database struct {
	*gorm.DB
	config *config.Config
}

func OpenDB(config *config.Config, logger *slog.Logger) (*Database, error) {
	gormcfg := &gorm.Config{
		NamingStrategy:  schema.NamingStrategy{SingularTable: true},
		PrepareStmt:     true,
		CreateBatchSize: config.BatchSize,
		Logger:          sloggorm.New(sloggorm.WithHandler(logger.Handler())),
	}

	instance, err := gorm.Open(postgres.Open(config.DSN), gormcfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := instance.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(config.MaxConns)
	sqlDB.SetMaxIdleConns(config.IdleConns)

	return &Database{DB: instance, config: config}, nil
}

func (d Database) Migrate() error {
	if !d.config.AutoMigrate {
		return nil
	}

	workDir, err := atlasexec.NewWorkingDir(
		atlasexec.WithMigrations(
			os.DirFS(d.config.MigrationDir),
		),
	)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := workDir.Close(); cerr != nil {
			if err == nil {
				err = cerr
			}
		}
	}()

	client, err := atlasexec.NewClient(workDir.Path(), "atlas")
	if err != nil {
		return err
	}

	if _, err := client.MigrateApply(context.Background(), &atlasexec.MigrateApplyParams{
		URL: fmt.Sprintf("%s?sslmode=disable", d.config.DSN),
	}); err != nil {
		return err
	}

	return d.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS tx_account_ids ON tx USING GIN ("account_ids")`).Error; err != nil {
			return err
		}

		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS tx_nft_ids ON tx USING GIN ("nft_ids")`).Error; err != nil {
			return err
		}

		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS tx_msg_type_ids ON tx USING GIN ("msg_type_ids")`).Error; err != nil {
			return err
		}

		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS tx_type_tag_ids ON tx USING GIN ("type_tag_ids")`).Error; err != nil {
			return err
		}

		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS evm_tx_account_ids ON evm_tx USING GIN ("account_ids")`).Error; err != nil {
			return err
		}

		return nil
	})
}

func (d Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d Database) GetBatchSize() int {
	return d.config.BatchSize
}
