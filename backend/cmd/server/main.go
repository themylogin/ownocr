package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/riverqueue/river"

	"github.com/themylogin/ownocr/backend/internal/books"
	"github.com/themylogin/ownocr/backend/internal/config"
	"github.com/themylogin/ownocr/backend/internal/db"
	"github.com/themylogin/ownocr/backend/internal/dewarp"
	"github.com/themylogin/ownocr/backend/internal/filestore"
	"github.com/themylogin/ownocr/backend/internal/httpapi"
	"github.com/themylogin/ownocr/backend/internal/ingest"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
	"github.com/themylogin/ownocr/backend/internal/ocr"
	"github.com/themylogin/ownocr/backend/internal/queue"
	"github.com/themylogin/ownocr/backend/internal/split"
	"github.com/themylogin/ownocr/backend/internal/ws"
)

const (
	httpDrainTimeout  = 30 * time.Second
	queueDrainTimeout = 60 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.WithMigrationLock(ctx, pool, func(ctx context.Context) error {
		if err := db.Migrate(ctx, pool); err != nil {
			return err
		}
		return queue.Migrate(ctx, pool)
	}); err != nil {
		return err
	}

	files, err := filestore.New(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("open file store: %w", err)
	}

	hub := ws.NewHub(cfg.AllowedOrigins)
	store := books.NewStore(pool)
	objects := objectstore.New(objectstore.Settings(cfg.S3))

	workers := river.NewWorkers()
	river.AddWorker(workers, ingest.NewWorker(store, files, hub, cfg.PDFDPI))
	river.AddWorker(workers, split.NewWorker(store, files, hub, split.NewClient(cfg.SidecarURL)))
	river.AddWorker(workers, dewarp.NewWorker(store, files, hub, objects, dewarp.Settings{
		Settings: objectstore.Settings(cfg.S3),
		APIURL:   cfg.Dewarp.APIURL,
		APIToken: cfg.Dewarp.APIToken,
	}))
	river.AddWorker(workers, ocr.NewWorker(store, files, hub, objects, ocr.Settings{
		Settings: objectstore.Settings(cfg.S3),
		APIURL:   cfg.OCR.APIURL,
		APIToken: cfg.OCR.APIToken,
	}))

	queueClient, err := queue.New(pool, workers, cfg.Workers)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}

	if err := queueClient.Start(ctx); err != nil {
		listener.Close()
		return err
	}

	server := &http.Server{
		Handler: httpapi.NewRouter(httpapi.Deps{
			Pool:           pool,
			Hub:            hub,
			Files:          files,
			Books:          store,
			Queue:          queueClient,
			MaxUploadBytes: cfg.MaxUploadBytes,
			Objects:        objects,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", listener.Addr().String())
		errs <- server.Serve(listener)
	}()

	var serveErr error
	select {
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	case <-ctx.Done():
		stop()
	}

	slog.Info("shutting down")

	httpCtx, cancelHTTP := context.WithTimeout(context.WithoutCancel(ctx), httpDrainTimeout)
	defer cancelHTTP()

	queueCtx, cancelQueue := context.WithTimeout(context.WithoutCancel(ctx), queueDrainTimeout)
	defer cancelQueue()

	return errors.Join(serveErr, server.Shutdown(httpCtx), stopQueue(queueCtx, queueClient))
}

// River jobs are idempotent and will be restarted on the next startup.
func stopQueue(ctx context.Context, client *queue.Client) error {
	if err := client.Stop(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}
