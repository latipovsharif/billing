package base

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PGXDB is satisfied by *pgxpool.Pool and pgx.Tx alike, so repo functions work
// inside or outside a transaction.
type PGXDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Beginner can start a transaction (satisfied by *pgxpool.Pool).
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// IsNotFound reports whether err means "no rows".
func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// WithTx runs fn in a transaction, committing on success and rolling back on
// error or panic. All multi-write billing operations MUST use this.
func WithTx(ctx context.Context, db Beginner, fn func(tx pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
