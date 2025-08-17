package config

import "errors"

type Config struct {
	DSN          string
	AutoMigrate  bool
	MaxConns     int
	IdleConns    int
	BatchSize    int
	MigrationDir string
}

func (c Config) Validate() error {
	if c.DSN == "" {
		return errors.New("DB_DSN is required")
	}
	if c.MaxConns < 0 {
		return errors.New("DB_MAX_CONNS is invalid")
	}
	if c.IdleConns < 1 {
		return errors.New("DB_IDLE_CONNS is invalid")
	}
	if c.BatchSize < 1 {
		return errors.New("DB_BATCH_SIZE is invalid")
	}
	if c.MigrationDir == "" {
		return errors.New("DB_MIGRATION_DIR is required")
	}
	// no check AutoMigrate
	return nil
}
