package db

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationLock = 4242

// WithMigrationLock serializes replicas that start together. Two of them
// issuing CREATE TABLE IF NOT EXISTS at the same moment is a duplicate key
// error in Postgres, so the lock covers the whole sequence, not one statement.
func WithMigrationLock(ctx context.Context, pool *pgxpool.Pool, migrate func(context.Context) error) error {
	// The lock lives on one connection, so it must be held, not borrowed from
	// the pool for each statement. Release alone does not drop it.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		return fmt.Errorf("take migration lock: %w", err)
	}
	defer func() {
		unlock := context.WithoutCancel(ctx)
		if _, err := conn.Exec(unlock, "SELECT pg_advisory_unlock($1)", migrationLock); err != nil {
			slog.Warn("release migration lock", "error", err)
		}
	}()

	return migrate(ctx)
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		if err := applyMigration(ctx, pool, name); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}

	return nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, name string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var applied bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)", name).Scan(&applied); err != nil {
		return err
	}
	if applied {
		return tx.Rollback(ctx)
	}

	statements, err := migrationFiles.ReadFile("migrations/" + name)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, string(statements)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (name) VALUES ($1)", name); err != nil {
		return err
	}

	slog.Info("migration applied", "name", name)
	return tx.Commit(ctx)
}
