// Package testdb hands every test package the same database without letting
// them tread on each other. `go test ./...` runs packages in parallel, and each
// of them empties the book table, so they take turns instead.
package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// One lock for the whole suite. A test holds it from its first query to its
// cleanup, so no other package can empty the table underneath it.
const suiteLock = 4243

// New answers a pool on an empty book table, or skips the test when no database
// is configured.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	// The lock lives on one connection, so it must be held, not borrowed from
	// the pool for each statement.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", suiteLock); err != nil {
		t.Fatalf("lock: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", suiteLock)
		conn.Release()
	})

	if _, err := conn.Exec(ctx, "TRUNCATE book CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return pool
}
