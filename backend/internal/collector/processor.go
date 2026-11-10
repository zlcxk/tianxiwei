package collector

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

type Processor struct {
	Store *store.Store
}

func (p *Processor) Run(parent context.Context, job models.Job, source models.Source) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	log := slog.With("job_id", job.ID, "source", source.Name)
	adapter, err := NewAdapter(source)
	if err != nil {
		log.Error("adapter unavailable", "error", err)
		_ = p.Store.CompleteJob(context.Background(), job.ID, source.ID, 0, 0, 0, 0, err.Error(), true)
		return
	}
	if err := adapter.Validate(); err != nil {
		_ = p.Store.CompleteJob(context.Background(), job.ID, source.ID, 0, 0, 0, 0, err.Error(), true)
		return
	}
	items, _, err := adapter.Fetch(ctx, "")
	if err != nil {
		log.Warn("collection failed", "error", err)
		_ = p.Store.CompleteJob(context.Background(), job.ID, source.ID, 0, 0, 0, 0, err.Error(), true)
		return
	}
	rules, err := p.Store.ListRules(ctx)
	if err != nil {
		log.Error("load classification rules", "error", err)
		_ = p.Store.CompleteJob(context.Background(), job.ID, source.ID, len(items), 0, 0, 0, "加载分类规则失败: "+err.Error(), true)
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
	if failed > 0 {
		errMsg = "部分条目处理失败: " + strings.Join(itemErrors, "; ")
	}
	if err := p.Store.CompleteJob(context.Background(), job.ID, source.ID, fetched, inserted, duplicates, failed, errMsg, false); err != nil {
		log.Error("complete job", "error", err)
		return
	}
	log.Info("collection finished", "fetched", fetched, "inserted", inserted, "duplicates", duplicates, "failed", failed)
}
