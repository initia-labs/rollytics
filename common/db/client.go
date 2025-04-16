package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	pool   *pgxpool.Pool
	config *Config
}

func NewClient(config *Config) (*Client, error) {
	pool, err := pgxpool.New(context.Background(), config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	return &Client{pool: pool, config: config}, nil
}

func (c *Client) Close() error {
	c.pool.Close()
	return nil
}

func (c *Client) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	return c.pool.QueryRow(ctx, sql, arguments...)
}

func (c *Client) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	return c.pool.Query(ctx, sql, arguments...)
}

func (c *Client) Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	return c.pool.Exec(ctx, sql, arguments...)
}

func (c *Client) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	tx, err := c.pool.BeginTx(ctx, txOptions)
	if err != nil {
		return nil, err
	}

	return &Tx{
		Tx:     tx,
		config: c.config,
	}, nil
}

var _ pgx.Tx = &Tx{}

type Tx struct {
	pgx.Tx
	config *Config
}

func (t *Tx) Commit(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, t.config.Timeout)
	defer cancel()

	return t.Tx.Commit(ctx)
}

func (t *Tx) Rollback(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, t.config.Timeout)
	defer cancel()

	return t.Tx.Rollback(ctx)
}
