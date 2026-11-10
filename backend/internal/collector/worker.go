package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

const (
	tickInterval  = 30 * time.Second
	maxConcurrent = 2
	shutdownWait  = 15 * time.Second
)

type Worker struct {
	Store           *store.Store
	Processor       *Processor
	SearchProcessor *SearchProcessor
	ID              string
	sem             chan struct{}
	wg              sync.WaitGroup
	// preferSearch alternates after every successful claim so that a busy
	// collection queue cannot starve search runs (and vice versa).
	preferSearch bool
}

func NewWorker(s *store.Store) *Worker {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "worker"
	}
	id := fmt.Sprintf("%s-%d", host, os.Getpid())
	w := &Worker{
		Store:           s,
		Processor:       &Processor{Store: s, WorkerID: id},
		SearchProcessor: NewSearchProcessor(s),
		ID:              id,
		sem:             make(chan struct{}, maxConcurrent),
	}
	w.SearchProcessor.WorkerID = id
	return w
}

func (w *Worker) Run(ctx context.Context) {
	slog.Info("collection worker started",
		"worker_id", w.ID,
		"version", buildVersion(),
		"tick_interval", tickInterval.String(),
		"max_concurrent", maxConcurrent)

	w.tick(ctx)
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("collection worker stopping", "worker_id", w.ID)
			w.waitForInflight()
			slog.Info("collection worker stopped", "worker_id", w.ID)
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// waitForInflight gives running tasks a short window to observe cancellation
// and persist their final state before the process closes the database pool.
func (w *Worker) waitForInflight() {
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownWait):
		slog.Warn("worker shutdown timed out waiting for in-flight tasks",
			"worker_id", w.ID, "wait", shutdownWait.String())
	}
}

func (w *Worker) tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if n, err := w.Store.RecoverStaleJobs(ctx); err != nil {
		slog.Warn("recover stale jobs", "worker_id", w.ID, "task_type", "collection", "error", err)
	} else if n > 0 {
		slog.Warn("recovered stale jobs", "worker_id", w.ID, "task_type", "collection", "count", n)
	}
	if n, err := w.Store.CreateDueJobs(ctx); err != nil {
		slog.Warn("schedule jobs", "worker_id", w.ID, "task_type", "collection", "error", err)
	} else if n > 0 {
		slog.Info("scheduled collection jobs", "worker_id", w.ID, "count", n)
	}
	if n, err := w.Store.RecoverStaleSearchRuns(ctx); err != nil {
		slog.Warn("recover stale search runs", "worker_id", w.ID, "task_type", "search", "error", err)
	} else if n > 0 {
		slog.Warn("recovered stale search runs", "worker_id", w.ID, "task_type", "search", "count", n)
	}
	if n, err := w.Store.CreateDueSearchRuns(ctx); err != nil {
		slog.Warn("schedule search runs", "worker_id", w.ID, "task_type", "search", "error", err)
	} else if n > 0 {
		slog.Info("scheduled search runs", "worker_id", w.ID, "count", n)
	}

	slog.Debug("scheduler tick", "worker_id", w.ID, "prefer_search", w.preferSearch)
	for i := 0; i < maxConcurrent; i++ {
		select {
		case w.sem <- struct{}{}:
		default:
			return
		}
		if !w.claimOne(ctx) {
			<-w.sem
			return
		}
	}
}

// claimOne tries the currently preferred task type first, then the other type.
// It returns false only when neither queue has claimable work. Database errors
// are logged and do not prevent the other task type from being tried.
func (w *Worker) claimOne(ctx context.Context) bool {
	first, second := "collection", "search"
	if w.preferSearch {
		first, second = second, first
	}
	for _, taskType := range []string{first, second} {
		switch taskType {
		case "collection":
			job, source, err := w.Store.ClaimJob(ctx)
			if err == nil {
				w.preferSearch = true
				w.launchCollection(ctx, job, source)
				return true
			}
			if !errors.Is(err, store.ErrNotFound) {
				slog.Warn("claim job failed", "worker_id", w.ID, "task_type", "collection", "error", err)
			}
		case "search":
			run, task, err := w.Store.ClaimSearchRun(ctx)
			if err == nil {
				w.preferSearch = false
				w.launchSearch(ctx, run, task)
				return true
			}
			if !errors.Is(err, store.ErrNotFound) {
				slog.Warn("claim search run failed", "worker_id", w.ID, "task_type", "search", "error", err)
			}
		}
	}
	return false
}

func (w *Worker) launchCollection(ctx context.Context, job models.Job, source models.Source) {
	slog.Info("task claimed", "worker_id", w.ID, "task_type", "collection",
		"job_id", job.ID, "source_id", source.ID, "source", source.Name)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer func() { <-w.sem }()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("task execution panicked", "worker_id", w.ID, "task_type", "collection",
					"job_id", job.ID, "panic", r, "stack", string(debug.Stack()))
				completeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = w.Store.CompleteJob(completeCtx, job.ID, source.ID, 0, 0, 0, 0,
					fmt.Sprintf("任务执行 panic: %v", r), true)
			}
		}()
		slog.Info("task execution started", "worker_id", w.ID, "task_type", "collection", "job_id", job.ID)
		w.Processor.Run(ctx, job, source)
	}()
}

func (w *Worker) launchSearch(ctx context.Context, run models.SearchRun, task models.SearchTask) {
	slog.Info("task claimed", "worker_id", w.ID, "task_type", "search",
		"run_id", run.ID, "search_task_id", task.ID, "task", task.Name)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer func() { <-w.sem }()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("task execution panicked", "worker_id", w.ID, "task_type", "search",
					"run_id", run.ID, "panic", r, "stack", string(debug.Stack()))
				completeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = w.Store.CompleteSearchRun(completeCtx, run.ID, task.ID, 0, 0, 0, 0, 0, 0,
					fmt.Sprintf("任务执行 panic: %v", r), true)
			}
		}()
		slog.Info("task execution started", "worker_id", w.ID, "task_type", "search", "run_id", run.ID)
		w.SearchProcessor.Run(ctx, run, task)
	}()
}

// Version can be injected at build time:
//
//	-ldflags "-X xiweispace/backend/internal/collector.Version=v1.0.0"
var Version = ""

func buildVersion() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}
