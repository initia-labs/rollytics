package orm

import (
	"log/slog"

	"github.com/initia-labs/rollytics/orm/config"
	"github.com/initia-labs/rollytics/types"
	sloggorm "github.com/orandin/slog-gorm"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
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
	logger *zap.Logger
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

	//db = db.WithContext((logger.WithContext(context.Background())))
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

	return d.AutoMigrate(&types.CollectedSeqInfo{}, &types.CollectedBlock{}, &types.CollectedTx{}, &types.CollectedAccountTx{})
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
