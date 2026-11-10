package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
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
	worker.Run(ctx)
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
