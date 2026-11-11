package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"xiweispace/backend/internal/collector"
	"xiweispace/backend/internal/config"
	"xiweispace/backend/internal/db"
	"xiweispace/backend/internal/store"
)

func main() {
	cfg := config.Load()
	setupLogger(cfg.Environment)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := connectWithRetry(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, cfg.SeedOnStart); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	worker := collector.NewWorker(store.New(pool))
	if cfg.SearchOverlap > 0 {
		worker.SearchProcessor.OverlapWindow = cfg.SearchOverlap
	}
	worker.Run(ctx)
}

// connectWithRetry keeps the worker alive while PostgreSQL is temporarily
// unavailable. db.Connect already retries for about a minute; this outer loop
// covers longer outages without exiting the process.
func connectWithRetry(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	for attempt := 1; ; attempt++ {
		pool, err := db.Connect(ctx, databaseURL)
		if err == nil {
			return pool, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Error("database connection failed, retrying", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func setupLogger(env string) {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}
