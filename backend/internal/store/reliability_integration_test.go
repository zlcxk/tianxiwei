package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"xiweispace/backend/internal/db"
)

// TestJobLifecycleReliabilityIntegration covers partial terminal states,
// heartbeat-based stale recovery and the "database error is not ErrNotFound"
// contract for both task types. It runs only against a disposable database.
func TestJobLifecycleReliabilityIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run store integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool, true); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	s := New(pool)
	tag := fmt.Sprintf("%d", time.Now().UnixNano())
	enabled := true
	source, err := s.CreateSource(ctx, SourceInput{
		Name:            "reliability-test-" + tag,
		Platform:        "test",
		SourceType:      "media",
		AdapterType:     "manual",
		TrustLevel:      "media",
		Enabled:         &enabled,
		IntervalMinutes: 1440,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	providers, err := s.ListSearchProviders(ctx)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if len(providers) == 0 {
		t.Skip("no seeded search provider")
	}
	task, err := s.CreateSearchTask(ctx, SearchTaskInput{
		Name: "reliability-task-" + tag, ProviderID: providers[0].ID, Query: "田曦薇",
		SourceType: "media", TrustLevel: "media", IntervalMinutes: 60, MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("create search task: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM search_runs WHERE task_id=$1::uuid`, task.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM search_tasks WHERE id=$1::uuid`, task.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM collection_jobs WHERE source_id=$1::uuid`, source.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM sources WHERE id=$1::uuid`, source.ID)
	})

	insertJob := func() string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO collection_jobs (source_id, status, trigger_type, created_at)
			VALUES ($1::uuid, 'queued', 'manual', now() - interval '2 hours')
			RETURNING id`, source.ID).Scan(&id); err != nil {
			t.Fatalf("insert job: %v", err)
		}
		return id
	}
	jobStatus := func(id string) (string, string, time.Time) {
		var status, errMsg string
		var heartbeat time.Time
		if err := pool.QueryRow(ctx, `SELECT status, error_message, heartbeat_at FROM collection_jobs WHERE id=$1::uuid`, id).
			Scan(&status, &errMsg, &heartbeat); err != nil {
			t.Fatalf("read job %s: %v", id, err)
		}
		return status, errMsg, heartbeat
	}

	// A task with item-level failures ends as partial, never success.
	jobID := insertJob()
	claimed, _, err := s.ClaimJob(ctx)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	if claimed.ID != jobID {
		t.Fatalf("claimed %s, want oldest queued job %s", claimed.ID, jobID)
	}
	if status, _, heartbeat := jobStatus(jobID); status != "running" || heartbeat.IsZero() {
		t.Fatalf("claimed job status=%q heartbeat=%v, want running with heartbeat", status, heartbeat)
	}
	if err := s.CompleteJob(ctx, jobID, source.ID, 5, 3, 1, 1, "one item failed", false); err != nil {
		t.Fatalf("complete partial job: %v", err)
	}
	if status, _, _ := jobStatus(jobID); status != "partial" {
		t.Fatalf("job status = %q, want partial", status)
	}

	// A task without item failures stays success.
	successID := insertJob()
	claimed, _, err = s.ClaimJob(ctx)
	if err != nil {
		t.Fatalf("claim success job: %v", err)
	}
	if claimed.ID != successID {
		t.Fatalf("claimed %s, want %s", claimed.ID, successID)
	}
	if err := s.CompleteJob(ctx, successID, source.ID, 5, 5, 0, 0, "", false); err != nil {
		t.Fatalf("complete success job: %v", err)
	}
	if status, _, _ := jobStatus(successID); status != "success" {
		t.Fatalf("job status = %q, want success", status)
	}

	// Heartbeats keep a live task out of stale recovery; an old heartbeat is
	// reclaimed with an explicit reason.
	liveID := insertJob()
	if _, _, err := s.ClaimJob(ctx); err != nil {
		t.Fatalf("claim live job: %v", err)
	}
	if n, err := s.RecoverStaleJobs(ctx); err != nil {
		t.Fatalf("recover live jobs: %v", err)
	} else if n != 0 {
		t.Fatalf("recovered %d live jobs, want 0", n)
	}
	if status, _, _ := jobStatus(liveID); status != "running" {
		t.Fatalf("live job status = %q, want running", status)
	}
	if _, err := pool.Exec(ctx, `UPDATE collection_jobs SET heartbeat_at=now() - interval '9 minutes' WHERE id=$1::uuid`, liveID); err != nil {
		t.Fatalf("age job heartbeat: %v", err)
	}
	if _, err := s.RecoverStaleJobs(ctx); err != nil {
		t.Fatalf("recover stale jobs: %v", err)
	}
	if status, errMsg, _ := jobStatus(liveID); status != "failed" || !strings.Contains(errMsg, "心跳超时") {
		t.Fatalf("stale job status=%q error=%q, want failed with heartbeat reason", status, errMsg)
	}

	// Database errors must not be reported as "no task available".
	canceledCtx, cancelClaim := context.WithCancel(ctx)
	cancelClaim()
	if _, _, err := s.ClaimJob(canceledCtx); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("claim with canceled context = %v, want a database error distinct from ErrNotFound", err)
	}

	insertRun := func() string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO search_runs (task_id, status, trigger_type, created_at)
			VALUES ($1::uuid, 'queued', 'manual', now() - interval '2 hours')
			RETURNING id`, task.ID).Scan(&id); err != nil {
			t.Fatalf("insert search run: %v", err)
		}
		return id
	}
	runStatus := func(id string) (string, string) {
		var status, errMsg string
		if err := pool.QueryRow(ctx, `SELECT status, error_message FROM search_runs WHERE id=$1::uuid`, id).
			Scan(&status, &errMsg); err != nil {
			t.Fatalf("read search run %s: %v", id, err)
		}
		return status, errMsg
	}

	partialRunID := insertRun()
	run, claimedTask, err := s.ClaimSearchRun(ctx)
	if err != nil {
		t.Fatalf("claim search run: %v", err)
	}
	if run.ID != partialRunID || claimedTask.ID != task.ID {
		t.Fatalf("claimed run=%s task=%s, want run=%s task=%s", run.ID, claimedTask.ID, partialRunID, task.ID)
	}
	if err := s.CompleteSearchRun(ctx, partialRunID, task.ID, 5, 2, 1, 2, 3, 1, "two candidates failed", false); err != nil {
		t.Fatalf("complete partial search run: %v", err)
	}
	if status, _ := runStatus(partialRunID); status != "partial" {
		t.Fatalf("search run status = %q, want partial", status)
	}

	staleRunID := insertRun()
	if _, _, err := s.ClaimSearchRun(ctx); err != nil {
		t.Fatalf("claim stale search run: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE search_runs SET heartbeat_at=now() - interval '11 minutes' WHERE id=$1::uuid`, staleRunID); err != nil {
		t.Fatalf("age search heartbeat: %v", err)
	}
	if _, err := s.RecoverStaleSearchRuns(ctx); err != nil {
		t.Fatalf("recover stale search runs: %v", err)
	}
	if status, errMsg := runStatus(staleRunID); status != "failed" || !strings.Contains(errMsg, "心跳超时") {
		t.Fatalf("stale run status=%q error=%q, want failed with heartbeat reason", status, errMsg)
	}
	if _, _, err := s.ClaimSearchRun(canceledCtx); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("claim search run with canceled context = %v, want a database error distinct from ErrNotFound", err)
	}
}
