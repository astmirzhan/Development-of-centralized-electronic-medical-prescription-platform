// Package testutil provides isolated PostgreSQL 16 integration test helpers.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xjam-byte/erx-platform/backend/internal/seed"
)

var dbCounter atomic.Uint64

func defaultAdminDSN() string {
	if v := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
}

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "migrations"))
}

// ReadMigrationSQL reads either "000001_init_schema.up.sql" or "000001_init_schema.down.sql".
func ReadMigrationSQL(t *testing.T, filename string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(migrationsDir(), filename))
	if err != nil {
		t.Fatalf("read migration %s: %v", filename, err)
	}
	return string(raw)
}

// SetupMigratedDB creates an isolated temporary PostgreSQL 16 database, applies
// 000001_init_schema.up.sql, and drops the database on test cleanup.
func SetupMigratedDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	adminDSN := defaultAdminDSN()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		if os.Getenv("TEST_DATABASE_URL") == "" && os.Getenv("CI") == "" {
			t.Skipf("skipping PostgreSQL integration test: %v", err)
		}
		t.Fatalf("connect admin postgres: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		if os.Getenv("TEST_DATABASE_URL") == "" && os.Getenv("CI") == "" {
			t.Skipf("skipping PostgreSQL integration test (unreachable): %v", err)
		}
		t.Fatalf("ping admin postgres: %v", err)
	}

	randBuf := make([]byte, 4)
	_, _ = rand.Read(randBuf)
	dbName := fmt.Sprintf("erx_it_%d_%d_%s", os.Getpid(), dbCounter.Add(1), hex.EncodeToString(randBuf))
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		adminPool.Close()
		t.Fatalf("create isolated test database %s: %v", dbName, err)
	}

	testDSN, err := replaceDatabaseInDSN(adminDSN, dbName)
	if err != nil {
		_, _ = adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
		adminPool.Close()
		t.Fatalf("build test DSN: %v", err)
	}

	pool, err := pgxpool.New(ctx, testDSN)
	if err != nil {
		_, _ = adminPool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
		adminPool.Close()
		t.Fatalf("connect isolated test database %s: %v", dbName, err)
	}

	t.Cleanup(func() {
		pool.Close()
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		_, _ = adminPool.Exec(dropCtx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
		adminPool.Close()
	})

	upSQL := ReadMigrationSQL(t, "000001_init_schema.up.sql")
	if _, err := pool.Exec(ctx, upSQL); err != nil {
		t.Fatalf("apply up migration: %v", err)
	}

	return pool
}

// SetupSeededDB creates an isolated migrated PostgreSQL 16 database and runs the idempotent seeder.
func SetupSeededDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := SetupMigratedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := seed.Run(ctx, pool); err != nil {
		t.Fatalf("seed isolated database: %v", err)
	}
	return pool
}

func replaceDatabaseInDSN(dsn, dbName string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + dbName
	return u.String(), nil
}
