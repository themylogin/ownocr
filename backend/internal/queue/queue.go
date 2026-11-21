package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

type Client = river.Client[pgx.Tx]

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}

	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("apply river migrations: %w", err)
	}

	return nil
}

// Ingest rasterizes and resizes whole books, so it runs alone on its own queue.
// One saturating book must not starve every other queue or the HTTP server.
const IngestQueue = "ingest"

// Dewarp moves a whole book to object storage and back, for the same reason.
const DewarpQueue = "dewarp"

// Split rewrites every page of a book, for the same reason.
const SplitQueue = "split"

// OCR moves a whole book to object storage and reads it back, for the same
// reason as dewarp.
const OCRQueue = "ocr"

func New(pool *pgxpool.Pool, workers *river.Workers, maxWorkers int) (*Client, error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:  slog.Default(),
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: maxWorkers},
			IngestQueue:        {MaxWorkers: 1},
			DewarpQueue:        {MaxWorkers: 1},
			SplitQueue:         {MaxWorkers: 1},
			OCRQueue:           {MaxWorkers: 1},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}

	return client, nil
}
