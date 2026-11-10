package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	"xiweispace/backend/internal/db"
)

func testHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// TestInsertCollectedIntegration exercises the real INSERT used by the worker.
// It is skipped unless TEST_DATABASE_URL points at a disposable PostgreSQL database.
func TestInsertCollectedIntegration(t *testing.T) {
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
	defer pool.Close()

	if err := db.Migrate(ctx, pool, true); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	s := New(pool)
	enabled := true
	source, err := s.CreateSource(ctx, SourceInput{
		Name:            fmt.Sprintf("integration-test-%d", time.Now().UnixNano()),
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
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM contents WHERE source_id=$1::uuid`, source.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM sources WHERE id=$1::uuid`, source.ID)
	})

	base := CollectedContent{
		SourceID:      source.ID,
		SourceItemID:  "item-1",
		Title:         "集成测试条目",
		Summary:       "验证采集入库 SQL 与多层去重。",
		ContentType:   "media",
		Author:        "test",
		URL:           "https://example.com/news/1",
		URLHash:       testHash("https://example.com/news/1"),
		ContentHash:   testHash("集成测试条目\n验证采集入库 SQL 与多层去重。"),
		PublishedAt:   time.Now(),
		TrustLevel:    "media",
		IsOfficial:    false,
		ReviewStatus:  "none",
		ContentStatus: "published",
		RawData:       []byte(`{"test":true}`),
		SceneIDs:      []string{"20000000-0000-4000-8000-000000000001"},
	}

	status, err := s.InsertCollected(ctx, base)
	if err != nil {
		t.Fatalf("insert collected: %v", err)
	}
	if status != Inserted {
		t.Fatalf("first insert status = %q, want %q", status, Inserted)
	}

	sameItem := base
	sameItem.URL = "https://example.com/news/1-mirror"
	sameItem.URLHash = testHash(sameItem.URL)
	if status, err = s.InsertCollected(ctx, sameItem); err != nil || status != Duplicate {
		t.Fatalf("same source item: status=%q err=%v, want duplicate", status, err)
	}

	sameURL := base
	sameURL.SourceItemID = "item-2"
	if status, err = s.InsertCollected(ctx, sameURL); err != nil || status != Duplicate {
		t.Fatalf("same URL: status=%q err=%v, want duplicate", status, err)
	}

	sameContent := base
	sameContent.SourceItemID = "item-3"
	sameContent.URL = "https://example.com/news/3"
	sameContent.URLHash = testHash(sameContent.URL)
	if status, err = s.InsertCollected(ctx, sameContent); err != nil || status != Duplicate {
		t.Fatalf("same fingerprint: status=%q err=%v, want duplicate", status, err)
	}

	fresh := base
	fresh.SourceItemID = "item-4"
	fresh.Title = "集成测试条目二"
	fresh.Summary = "另一条独立内容。"
	fresh.URL = "https://example.com/news/4"
	fresh.URLHash = testHash(fresh.URL)
	fresh.ContentHash = testHash("集成测试条目二\n另一条独立内容。")
	if status, err = s.InsertCollected(ctx, fresh); err != nil || status != Inserted {
		t.Fatalf("fresh item: status=%q err=%v, want inserted", status, err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM contents WHERE source_id=$1::uuid`, source.ID).Scan(&count); err != nil {
		t.Fatalf("count contents: %v", err)
	}
	if count != 2 {
		t.Fatalf("content count = %d, want 2", count)
	}

	var raw string
	if err := pool.QueryRow(ctx, `
		SELECT raw_data::text FROM contents WHERE source_id=$1::uuid AND source_item_id='item-1'`,
		source.ID).Scan(&raw); err != nil {
		t.Fatalf("read raw_data: %v", err)
	}
	if raw != `{"test": true}` && raw != `{"test":true}` {
		t.Fatalf("unexpected raw_data: %s", raw)
	}
}
