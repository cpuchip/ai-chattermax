// Package db owns the Postgres connection pool and the boot-time migration
// runner. It is generic: the migration files are supplied as an fs.FS by the
// caller (the root package embeds them), so this package has no asset coupling.
package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open builds a pool from the DSN and waits (up to ~30s) for the database to
// accept connections — the pg container often starts a beat after the app.
// DSN parse errors are returned redacted (pgx echoes the password otherwise).
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL (redacted to avoid leaking credentials)")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("could not build pool from DATABASE_URL (redacted)")
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if time.Now().After(deadline) {
			pool.Close()
			return nil, fmt.Errorf("database not reachable after 30s: %w", err)
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Migrate applies every migrations/*.sql from fsys not yet recorded in
// schema_migrations, in lexical order, each in its own transaction. Idempotent.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists {
			continue
		}

		sqlBytes, err := fs.ReadFile(fsys, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if err := applyOne(ctx, pool, name, string(sqlBytes)); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, name, body string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", name, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	// A no-arg Exec uses the simple protocol, so the whole multi-statement
	// migration applies in one round trip.
	if _, err := tx.Exec(ctx, body); err != nil {
		return fmt.Errorf("apply %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("record %s: %w", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", name, err)
	}
	return nil
}

// ErrNoRows re-exports pgx.ErrNoRows so callers don't import pgx directly.
var ErrNoRows = pgx.ErrNoRows
