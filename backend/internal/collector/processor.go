package collector

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

const heartbeatInterval = 30 * time.Second

type Processor struct {
	Store    *store.Store
	WorkerID string
}

func (p *Processor) Run(parent context.Context, job models.Job, source models.Source) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	stopHeartbeat := p.startHeartbeat(job.ID)
	defer stopHeartbeat()

	log := slog.With("worker_id", p.WorkerID, "task_type", "collection",
		"job_id", job.ID, "source_id", source.ID, "source", source.Name)
	duration := func() string { return time.Since(started).Round(time.Millisecond).String() }

	fail := func(message string) {
		if err := p.Store.CompleteJob(context.Background(), job.ID, source.ID, 0, 0, 0, 0, message, true); err != nil {
			log.Error("complete failed collection job", "error", err)
		}
		log.Error("task execution failed", "status", "failed", "duration", duration(),
			"success_count", 0, "failure_count", 0, "error", message)
	}

	adapter, err := NewAdapter(source)
	if err != nil {
		fail("adapter unavailable: " + err.Error())
		return
	}
	if err := adapter.Validate(); err != nil {
		fail("adapter invalid: " + err.Error())
		return
	}
	items, _, err := adapter.Fetch(ctx, "")
	if err != nil {
		log.Warn("collection fetch failed", "error", err)
		fail(err.Error())
		return
	}
	rules, err := p.Store.ListRules(ctx)
	if err != nil {
		fail("加载分类规则失败: " + err.Error())
		return
	}

	fetched, inserted, duplicates, failed := len(items), 0, 0, 0
	itemErrors := make([]string, 0, 2)
	for _, item := range items {
		title := stripHTML(item.Title)
		summary := stripHTML(item.Summary)
		itemID := strings.TrimSpace(item.SourceItemID)
		if itemID == "" {
			itemID = strings.TrimSpace(item.URL)
		}
		if title == "" || itemID == "" {
			failed++
			continue
		}
		canonical := canonicalURL(item.URL)
		classification := Classify(title+" "+summary, rules)
		decision := DecideReview(source, title+" "+summary, len(classification.SceneIDs))
		ct := InferContentType(source, title+" "+summary)
		published := item.PublishedAt
		precision := "verified"
		if published == nil || published.After(time.Now().Add(24*time.Hour)) {
			// Never turn discovery time into publication time. Unknown dates stay
			// NULL and are ordered by discovered_at in the timeline.
			published = nil
			precision = "unknown"
		}
		author := item.Author
		if author == "" {
			author = source.Name
		}
		status, err := p.Store.InsertCollected(ctx, store.CollectedContent{
			SourceID:           source.ID,
			SourceItemID:       itemID,
			Title:              truncateRunes(title, 300),
			Summary:            truncateRunes(summary, 1200),
			ContentType:        ct,
			Author:             author,
			URL:                canonical,
			CoverURL:           item.CoverURL,
			URLHash:            hashString(canonical),
			ContentHash:        contentFingerprint(title, summary),
			PublishedAt:        published,
			PublishedPrecision: precision,
			TrustLevel:         source.TrustLevel,
			IsOfficial:         source.SourceType == "official" || source.TrustLevel == "official",
			ReviewStatus:       decision.ReviewStatus,
			ContentStatus:      decision.ContentStatus,
			ReviewReason:       decision.Reason,
			RawData:            item.Raw,
			SceneIDs:           classification.SceneIDs,
		})
		if err != nil {
			failed++
			log.Warn("insert item failed", "title", title, "error", err)
			if len(itemErrors) < 2 {
				itemErrors = append(itemErrors, err.Error())
			}
			continue
		}
		if status == store.Duplicate {
			duplicates++
		} else {
			inserted++
		}
	}

	errMsg := ""
	status := "success"
	if failed > 0 {
		errMsg = "部分条目处理失败: " + strings.Join(itemErrors, "; ")
		if len(itemErrors) == 0 {
			errMsg = "部分条目缺少必要字段，已跳过"
		}
		status = "partial"
	}
	if err := p.Store.CompleteJob(context.Background(), job.ID, source.ID, fetched, inserted, duplicates, failed, errMsg, false); err != nil {
		log.Error("complete job", "error", err)
		return
	}
	log.Info("collection finished", "status", status,
		"fetched", fetched, "inserted", inserted, "duplicates", duplicates, "failed", failed,
		"success_count", inserted, "failure_count", failed, "duration", duration())
}

func (p *Processor) startHeartbeat(jobID string) func() {
	return startHeartbeat(p.WorkerID, "collection", func(ctx context.Context) error {
		return p.Store.HeartbeatJob(ctx, jobID)
	})
}

// startHeartbeat refreshes the task lease while it is running. The returned
// stop function must be called before the task is completed.
func startHeartbeat(workerID, taskType string, beat func(context.Context) error) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := beat(ctx)
				cancel()
				if err != nil {
					slog.Warn("task heartbeat failed", "worker_id", workerID, "task_type", taskType, "error", err)
				}
			}
		}
	}()
	return func() { close(done) }
}
