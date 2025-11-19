package orm

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	defer func() { _ = workDir.Close() }()

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

// MigrateWithLastCheckResult contains information about the last migration
type MigrateWithLastCheckResult struct {
	LastMigrationHasTxModeNone bool
	RunLastMigration           func() error
}

// MigrateWithLastCheck checks migration status and conditionally handles the last migration
// If only 1 migration is pending, it checks if it has atlas:txmode none and returns info
// If more than 1 is pending, it runs all migrations normally
func (d Database) MigrateWithLastCheck() (*MigrateWithLastCheckResult, error) {
	if !d.config.AutoMigrate {
		return &MigrateWithLastCheckResult{
			LastMigrationHasTxModeNone: false,
			RunLastMigration:           func() error { return nil },
		}, nil
	}

	workDir, err := atlasexec.NewWorkingDir(
		atlasexec.WithMigrations(
			os.DirFS(d.config.MigrationDir),
		),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = workDir.Close() }()

	client, err := atlasexec.NewClient(workDir.Path(), "atlas")
	if err != nil {
		return nil, err
	}

	// Check migration status to see how many are pending
	status, err := client.MigrateStatus(context.Background(), &atlasexec.MigrateStatusParams{
		URL: d.config.DSN,
	})
	if err != nil {
		return nil, err
	}

	// Get all migration files to find the last one
	files, err := filepath.Glob(filepath.Join(d.config.MigrationDir, "*.sql"))
	if err != nil {
		return nil, err
	}

	var migrationFiles []string
	for _, f := range files {
		if !strings.Contains(f, "atlas.sum") {
			migrationFiles = append(migrationFiles, f)
		}
	}

	if len(migrationFiles) == 0 {
		return &MigrateWithLastCheckResult{
			LastMigrationHasTxModeNone: false,
			RunLastMigration:           func() error { return nil },
		}, nil
	}

	sort.Strings(migrationFiles)
	lastFile := migrationFiles[len(migrationFiles)-1]

	// Check if last migration has atlas:txmode none
	content, err := os.ReadFile(lastFile)
	if err != nil {
		return nil, err
	}
	hasTxModeNone := strings.Contains(string(content), "atlas:txmode none")

	// Count pending migrations
	pendingCount := len(status.Pending)

	// If only 1 migration is pending (the last one), we can handle it conditionally
	if pendingCount == 1 {
		return &MigrateWithLastCheckResult{
			LastMigrationHasTxModeNone: hasTxModeNone,
			RunLastMigration:           d.Migrate,
		}, nil
	}

	// If more than 1 pending, run all migrations normally
	if err := d.Migrate(); err != nil {
		return nil, err
	}

	return &MigrateWithLastCheckResult{
		LastMigrationHasTxModeNone: false,
		RunLastMigration:           func() error { return nil },
	}, nil
}
