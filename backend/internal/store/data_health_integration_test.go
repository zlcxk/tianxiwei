package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"xiweispace/backend/internal/db"
	"xiweispace/backend/internal/models"
)

// TestDataHealthIntegration verifies that completeness counts, image coverage
// and the filterable missing list stay consistent against a real database. It
// is skipped unless TEST_DATABASE_URL points at a disposable PostgreSQL.
func TestDataHealthIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run store integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	var artistID string
	if err := pool.QueryRow(ctx, `SELECT id FROM artists ORDER BY created_at LIMIT 1`).Scan(&artistID); err != nil {
		t.Fatalf("read artist: %v", err)
	}
	var contentID string
	err = pool.QueryRow(ctx, `
		INSERT INTO contents (artist_id, title, summary, content_type, source_url,
			cover_url, published_at, trust_level, content_status)
		VALUES ($1::uuid, '资料完善集成测试', '', 'other', '', '', NULL, 'internal', 'published')
		RETURNING id`, artistID).Scan(&contentID)
	if err != nil {
		t.Fatalf("insert content: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM contents WHERE id=$1::uuid`, contentID)
	})

	all, err := s.DataHealth(ctx, "all", dataHealthLimit)
	if err != nil {
		t.Fatalf("data health all: %v", err)
	}
	if all.Filter != "all" || all.Total < all.Returned || all.Returned != len(all.Items) {
		t.Fatalf("unexpected health envelope: %+v", all)
	}
	if all.Counts.PublishedContents == 0 {
		t.Fatal("seed has no published contents")
	}
	for _, item := range all.Items {
		if item.EditPath == "" || !strings.HasPrefix(item.EditPath, "/admin") {
			t.Fatalf("item %s has no admin edit path: %+v", item.ID, item)
		}
		if len(item.Missing) == 0 {
			t.Fatalf("item %s was listed without missing fields", item.ID)
		}
	}
	for name, pct := range map[string]float64{
		"artist avatar":    all.Coverage.ArtistAvatar,
		"artist cover":     all.Coverage.ArtistCover,
		"work cover":       all.Coverage.WorkCover,
		"character avatar": all.Coverage.CharacterAvatar,
		"content cover":    all.Coverage.ContentCover,
		"event cover":      all.Coverage.EventCover,
	} {
		if pct < 0 || pct > 100 {
			t.Fatalf("%s coverage out of range: %v", name, pct)
		}
	}

	// Every filter must agree with its count and only return matching rows.
	for filter, labels := range map[string][]string{
		"image":      {"头像", "封面"},
		"summary":    {"简介"},
		"date":       {"日期"},
		"relation":   {"关联内容"},
		"source_url": {"来源链接"},
	} {
		got, err := s.DataHealth(ctx, filter, dataHealthLimit)
		if err != nil {
			t.Fatalf("filter %s: %v", filter, err)
		}
		if got.Filter != filter || got.Returned != len(got.Items) {
			t.Fatalf("filter %s envelope = %+v", filter, got)
		}
		for _, item := range got.Items {
			if !hasAnyLabel(item.Missing, labels) {
				t.Fatalf("filter %s returned item without %v: %+v", filter, labels, item)
			}
		}
		switch filter {
		case "image":
			if got.Total != got.Counts.MissingImages {
				t.Fatalf("image total %d != missing_images %d", got.Total, got.Counts.MissingImages)
			}
			if got.Counts.MissingContentCover == 0 {
				t.Fatal("expected the new empty content to miss a cover")
			}
		case "summary":
			if got.Total != got.Counts.MissingSummaries {
				t.Fatalf("summary total %d != missing_summaries %d", got.Total, got.Counts.MissingSummaries)
			}
		case "date":
			if got.Total != got.Counts.MissingDates {
				t.Fatalf("date total %d != missing_dates %d", got.Total, got.Counts.MissingDates)
			}
		case "relation":
			if got.Total != got.Counts.MissingRelations {
				t.Fatalf("relation total %d != missing_relations %d", got.Total, got.Counts.MissingRelations)
			}
		case "source_url":
			if got.Total != got.Counts.MissingSourceURLs {
				t.Fatalf("source_url total %d != missing_source_urls %d", got.Total, got.Counts.MissingSourceURLs)
			}
		}
		if !containsDataHealthItem(got.Items, contentID) {
			t.Fatalf("filter %s should include the empty content %s", filter, contentID)
		}
	}

	// Fill every non-relation field; the row must disappear from those filters
	// and remain only under "relation" until a work link is added.
	if _, err := pool.Exec(ctx, `
		UPDATE contents
		SET cover_url='https://example.com/cover.jpg', summary='摘要',
		    source_url='https://example.com/article', published_at=now()
		WHERE id=$1::uuid`, contentID); err != nil {
		t.Fatalf("fill content fields: %v", err)
	}
	for _, filter := range []string{"image", "summary", "date", "source_url"} {
		got, err := s.DataHealth(ctx, filter, dataHealthLimit)
		if err != nil {
			t.Fatalf("filter %s after fill: %v", filter, err)
		}
		if containsDataHealthItem(got.Items, contentID) {
			t.Fatalf("filled content still missing under %s: %+v", filter, got.Items)
		}
	}
	relation, err := s.DataHealth(ctx, "relation", dataHealthLimit)
	if err != nil {
		t.Fatalf("relation filter after fill: %v", err)
	}
	if !containsDataHealthItem(relation.Items, contentID) {
		t.Fatal("content without any relation should be flagged")
	}
	for _, item := range relation.Items {
		if item.ID == contentID && item.EditPath != "/admin/contents?edit="+contentID {
			t.Fatalf("content edit path = %q", item.EditPath)
		}
	}

	var workID string
	if err := pool.QueryRow(ctx, `SELECT id FROM works ORDER BY created_at LIMIT 1`).Scan(&workID); err != nil {
		t.Fatalf("read work: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO content_works (content_id, work_id) VALUES ($1::uuid, $2::uuid)`, contentID, workID); err != nil {
		t.Fatalf("link work: %v", err)
	}
	relation, err = s.DataHealth(ctx, "relation", dataHealthLimit)
	if err != nil {
		t.Fatalf("relation filter after link: %v", err)
	}
	if containsDataHealthItem(relation.Items, contentID) {
		t.Fatal("content with a work relation should no longer be flagged")
	}

	if _, err := s.DataHealth(ctx, "unknown", dataHealthLimit); err != nil {
		t.Fatalf("unknown filter should fall back to all: %v", err)
	}
}

func hasAnyLabel(missing, labels []string) bool {
	for _, missingLabel := range missing {
		for _, label := range labels {
			if missingLabel == label {
				return true
			}
		}
	}
	return false
}

func containsDataHealthItem(items []models.DataHealthItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
