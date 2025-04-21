package config

import "errors"

type Config struct {
	DSN         string
	AutoMigrate bool
	MaxConns    int
	IdleConns   int
	BatchSize   int
}

func (c Config) Validate() error {
	if c.DSN == "" {
		return errors.New("DB_DSN is required")
	}
	if c.MaxConns < 1 {
		return errors.New("DB_MAX_CONNS is invalid")
	}
	if c.IdleConns < 1 {
		return errors.New("DB_IDLE_CONNS is invalid")
	}
	if c.BatchSize < 1 {
		return errors.New("DB_BATCH_SIZE is invalid")
	}
	// no check AutoMigrate
	return nil
}
