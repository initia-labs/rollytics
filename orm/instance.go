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

func (d Database) Migrate(ctx context.Context) error {
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

	if _, err := client.MigrateApply(ctx, &atlasexec.MigrateApplyParams{
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

// CheckLastMigrationConcurrency checks migration status and conditionally handles the last migration
// If only 1 migration is pending, it checks if it has atlas:txmode none and returns info
// If more than 1 is pending, it runs all migrations normally
func (d Database) CheckLastMigrationConcurrency(ctx context.Context) (bool, error) {
	if !d.config.AutoMigrate {
		return false, nil
	}

	workDir, err := atlasexec.NewWorkingDir(
		atlasexec.WithMigrations(
			os.DirFS(d.config.MigrationDir),
		),
	)
	if err != nil {
		return false, err
	}
	defer func() { _ = workDir.Close() }()

	client, err := atlasexec.NewClient(workDir.Path(), "atlas")
	if err != nil {
		return false, err
	}

	// Check migration status to see how many are pending
	status, err := client.MigrateStatus(ctx, &atlasexec.MigrateStatusParams{
		URL: d.config.DSN,
	})
	if err != nil {
		return false, err
	}

	// Get all migration files to find the last one
	files, err := filepath.Glob(filepath.Join(d.config.MigrationDir, "*.sql"))
	if err != nil {
		return false, err
	}

	var migrationFiles []string
	for _, f := range files {
		if !strings.Contains(f, "atlas.sum") {
			migrationFiles = append(migrationFiles, f)
		}
	}

	if len(migrationFiles) == 0 {
		return false, nil
	}

	sort.Strings(migrationFiles)
	lastFile := migrationFiles[len(migrationFiles)-1]

	// Check if last migration has atlas:txmode none
	content, err := os.ReadFile(lastFile) // #nosec G304 -- file path comes from filepath.Glob within MigrationDir
	if err != nil {
		return false, err
	}

	// If only 1 migration is pending (the last one) and it has atlas:txmode none, we can run it concurrently with indexer
	if len(status.Pending) == 1 && strings.Contains(string(content), "atlas:txmode none") {
		return true, nil
	}

	if err := d.Migrate(ctx); err != nil {
		return false, err
	}

	return false, nil
}
