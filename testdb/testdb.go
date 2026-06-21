// Package testdb gives integration tests a migrated billing DB plus a
// transaction-per-test that rolls back automatically. Tests skip when
// DATABASE_ROOT_PASS is unset or the DB is unreachable.
package testdb

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	poolOnce   sync.Once
	sharedPool *pgxpool.Pool
	poolErr    error
)

func dsn() string {
	pass := os.Getenv("DATABASE_ROOT_PASS")
	ssl := os.Getenv("DATABASE_SSL_MODE")
	if ssl == "" {
		ssl = "disable"
	}
	return fmt.Sprintf("postgres://postgres:%s@localhost:5432/billing?sslmode=%s", pass, ssl)
}

func initPool() {
	pass := os.Getenv("DATABASE_ROOT_PASS")
	m, err := migrate.New("file://../migrations", "pgx5://postgres:"+pass+"@localhost:5432/billing?sslmode=disable")
	if err != nil {
		poolErr = err
		return
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		poolErr = err
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		poolErr = err
		return
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		poolErr = err
		return
	}
	sharedPool = pool
}

// PoolOrSkip returns a migrated pool, or skips when unavailable.
func PoolOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("DATABASE_ROOT_PASS") == "" {
		t.Skip("DATABASE_ROOT_PASS not set, skipping integration test")
	}
	poolOnce.Do(initPool)
	if poolErr != nil {
		t.Skipf("cannot prepare billing DB: %v", poolErr)
	}
	return sharedPool
}

// TxOrSkip returns a context + open tx rolled back via t.Cleanup (full isolation).
func TxOrSkip(t *testing.T) (context.Context, pgx.Tx) {
	t.Helper()
	pool := PoolOrSkip(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return ctx, tx
}
