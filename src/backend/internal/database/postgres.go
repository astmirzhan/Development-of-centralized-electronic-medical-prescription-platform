// Package database manages the PostgreSQL 16 connection pool and transactions.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const healthPingTimeout = 2 * time.Second

// NewPool initializes a pgxpool.Pool and verifies connectivity.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := HealthPing(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("initial database ping: %w", err)
	}

	return pool, nil
}

// HealthPing verifies PostgreSQL reachability within a 2-second timeout.
func HealthPing(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil")
	}
	pingCtx, cancel := context.WithTimeout(ctx, healthPingTimeout)
	defer cancel()
	return pool.Ping(pingCtx)
}

// WithTx executes fn inside a PostgreSQL transaction, rolling back on error or panic.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(context.Background())
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(context.Background()); rbErr != nil {
			return fmt.Errorf("%w (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
