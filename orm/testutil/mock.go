package testutil

import (
	"log/slog"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/initia-labs/rollytics/orm"
)

func NewMockDB(logger *slog.Logger) (*orm.Database, sqlmock.Sqlmock, error) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		return nil, nil, err
	}

	gormcfg := &gorm.Config{
		NamingStrategy:  schema.NamingStrategy{SingularTable: true},
		PrepareStmt:     false,
		CreateBatchSize: 100,
		Logger:          nil,
	}

	instance, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), gormcfg)
	if err != nil {
		return nil, nil, err
	}


	return &orm.Database{
		DB:     instance,
	}, mock, nil
}
