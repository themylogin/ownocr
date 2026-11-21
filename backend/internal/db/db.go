package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	connectAttempts = 30
	connectDelay    = time.Second
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	for attempt := 1; attempt <= connectAttempts; attempt++ {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}

		slog.Warn("database not ready", "attempt", attempt, "error", err)

		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(connectDelay):
		}
	}

	pool.Close()
	return nil, fmt.Errorf("database unreachable after %d attempts: %w", connectAttempts, err)
}
