package orm

import (
	"context"
	"database/sql"
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
	"github.com/initia-labs/rollytics/orm/plugins"
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

	// Register custom metrics plugin
	metricsPlugin := plugins.NewMetricsPlugin()
	if err := instance.Use(metricsPlugin); err != nil {
		return nil, err
	}

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
	defer workDir.Close()

	client, err := atlasexec.NewClient(workDir.Path(), "atlas")
	if err != nil {
		return err
	}

	if _, err := client.MigrateApply(context.Background(), &atlasexec.MigrateApplyParams{
		URL: d.config.DSN,
	}); err != nil {
		return err
	}

	return nil
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

// GetDBStats returns database connection pool statistics
func (d Database) GetDBStats() (*sql.DBStats, error) {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return nil, err
	}

	stats := sqlDB.Stats()
	return &stats, nil
}
