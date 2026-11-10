package collector

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"xiweispace/backend/internal/store"
)

type Worker struct {
	Store           *store.Store
	Processor       *Processor
	SearchProcessor *SearchProcessor
	sem             chan struct{}
}

func NewWorker(s *store.Store) *Worker {
	return &Worker{
		Store:           s,
		Processor:       &Processor{Store: s},
		SearchProcessor: NewSearchProcessor(s),
		sem:             make(chan struct{}, 2),
	}
}

func (w *Worker) Run(ctx context.Context) {
	slog.Info("collection worker started")
	w.tick(ctx)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("collection worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	if n, err := w.Store.RecoverStaleJobs(ctx); err != nil {
		slog.Warn("recover stale jobs", "error", err)
	} else if n > 0 {
		slog.Warn("recovered stale jobs", "count", n)
	}
	if n, err := w.Store.CreateDueJobs(ctx); err != nil {
		slog.Warn("schedule jobs", "error", err)
	} else if n > 0 {
		slog.Info("scheduled collection jobs", "count", n)
	}
	if n, err := w.Store.RecoverStaleSearchRuns(ctx); err != nil {
		slog.Warn("recover stale search runs", "error", err)
	} else if n > 0 {
		slog.Warn("recovered stale search runs", "count", n)
	}
	if n, err := w.Store.CreateDueSearchRuns(ctx); err != nil {
		slog.Warn("schedule search runs", "error", err)
	} else if n > 0 {
		slog.Info("scheduled search runs", "count", n)
	}

	for i := 0; i < 2; i++ {
		select {
		case w.sem <- struct{}{}:
		default:
			return
		}
		job, source, err := w.Store.ClaimJob(ctx)
		if err == nil {
			go func() {
				defer func() { <-w.sem }()
				w.Processor.Run(ctx, job, source)
			}()
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			slog.Warn("claim job", "error", err)
		}
		run, task, searchErr := w.Store.ClaimSearchRun(ctx)
		if searchErr == nil {
			go func() {
				defer func() { <-w.sem }()
				w.SearchProcessor.Run(ctx, run, task)
			}()
			continue
		}
		<-w.sem
		if !errors.Is(searchErr, store.ErrNotFound) {
			slog.Warn("claim search run", "error", searchErr)
		}
		return
	}
}
