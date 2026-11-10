package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"xiweispace/backend/internal/db"
)

// TestDiscoveryTimelineIntegration covers the search-candidate insert path, the
// publication/discovery time split, stable cursor ordering, and event linking.
// It is skipped unless TEST_DATABASE_URL points at a disposable database.
func TestDiscoveryTimelineIntegration(t *testing.T) {
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
	artist, err := s.GetArtist(ctx)
	if err != nil {
		t.Fatalf("get seeded artist: %v", err)
	}
	providers, err := s.ListSearchProviders(ctx)
	if err != nil || len(providers) == 0 {
		t.Fatalf("list providers: %v (n=%d)", err, len(providers))
	}
	taskName := fmt.Sprintf("timeline-test-%d", time.Now().UnixNano())
	task, err := s.CreateSearchTask(ctx, SearchTaskInput{
		Name: taskName, ProviderID: providers[0].ID, Query: "田曦薇",
		SourceType: "media", TrustLevel: "media", IntervalMinutes: 60, MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	event, err := s.CreateEvent(ctx, artist.ID, EventInput{
		Title: "集成测试事件 " + taskName, EventDate: "2026-11-10", Status: "published",
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	prefix := fmt.Sprintf("TimelineTest-%d", time.Now().UnixNano())
	urlBase := fmt.Sprintf("https://example.com/timeline/%d", time.Now().UnixNano())
	publishedNewest := time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC)
	publishedOlder := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM contents WHERE title LIKE $1`, prefix+"%")
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM events WHERE id=$1::uuid`, event.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM search_tasks WHERE id=$1::uuid`, task.ID)
	})

	insert := func(suffix string, published *time.Time, precision string) string {
		url := urlBase + "/" + suffix
		status, err := s.InsertDiscovered(ctx, DiscoveredContent{
			SearchTaskID: task.ID, Title: prefix + "-" + suffix, Summary: "integration",
			ContentType: "news", Author: "test", URL: url, URLHash: testHash(url),
			PublishedAt: published, PublishedPrecision: precision,
			OriginName: "Example News", OriginPlatform: "example.com", OriginType: "media",
			MetadataStatus: "ok", TrustLevel: "media", ReviewStatus: "approved", ContentStatus: "published",
			RawData: []byte(`{"test":true}`),
		})
		if err != nil {
			t.Fatalf("insert discovered %s: %v", suffix, err)
		}
		if status != Inserted {
			t.Fatalf("insert discovered %s status = %q, want %q", suffix, status, Inserted)
		}
		items, _, err := s.ListContents(ctx, ContentFilter{Keyword: prefix + "-" + suffix, Limit: 1})
		if err != nil || len(items) != 1 {
			t.Fatalf("read discovered %s: %v (n=%d)", suffix, err, len(items))
		}
		return items[0].ID
	}

	newestID := insert("newest", &publishedNewest, "verified")
	olderID := insert("older", &publishedOlder, "estimated")
	unknownID := insert("unknown", nil, "unknown")

	items, total, err := s.ListContents(ctx, ContentFilter{Keyword: prefix, Limit: 2})
	if err != nil {
		t.Fatalf("list timeline: %v", err)
	}
	if total != 3 || len(items) != 2 {
		t.Fatalf("first page total=%d n=%d, want total=3 n=2", total, len(items))
	}
	if items[0].ID != newestID || items[1].ID != olderID {
		t.Fatalf("first page order = %s,%s, want %s,%s", items[0].ID, items[1].ID, newestID, olderID)
	}
	if items[1].PublishedPrecision != "estimated" {
		t.Fatalf("older precision = %q, want estimated", items[1].PublishedPrecision)
	}

	items, _, err = s.ListContents(ctx, ContentFilter{
		Keyword: prefix, Limit: 2,
		Cursor: &ContentCursor{EffectiveAt: items[1].EffectiveAt, CreatedAt: items[1].CreatedAt, ID: items[1].ID},
	})
	if err != nil {
		t.Fatalf("list timeline cursor: %v", err)
	}
	if len(items) != 1 || items[0].ID != unknownID {
		t.Fatalf("cursor page = %+v, want only unknown %s", items, unknownID)
	}
	unknown := items[0]
	if unknown.PublishedAt != nil {
		t.Fatalf("unknown published_at = %v, want NULL", *unknown.PublishedAt)
	}
	if unknown.PublishedPrecision != "unknown" {
		t.Fatalf("unknown precision = %q, want unknown", unknown.PublishedPrecision)
	}
	if unknown.DiscoveredAt.IsZero() {
		t.Fatal("unknown discovered_at must be set")
	}
	if unknown.OriginName != "Example News" || unknown.OriginPlatform != "example.com" {
		t.Fatalf("origin = %q/%q, want Example News/example.com", unknown.OriginName, unknown.OriginPlatform)
	}

	eventIDs := []string{event.ID}
	if _, err := s.UpdateContent(ctx, newestID, "", ContentPatch{EventIDs: &eventIDs}, "test link event"); err != nil {
		t.Fatalf("link event: %v", err)
	}
	if _, err := s.UpdateContent(ctx, unknownID, "", ContentPatch{EventIDs: &eventIDs}, "test link event"); err != nil {
		t.Fatalf("link event: %v", err)
	}
	eventDetail, eventContents, err := s.EventDetail(ctx, event.ID, "")
	if err != nil {
		t.Fatalf("event detail: %v", err)
	}
	if eventDetail.ContentCount != 2 || len(eventContents) != 2 {
		t.Fatalf("event count=%d n=%d, want 2", eventDetail.ContentCount, len(eventContents))
	}
	if eventContents[0].PublishedAt == nil || !eventContents[0].PublishedAt.Equal(publishedNewest) {
		t.Fatalf("event linking must not modify published_at: %+v", eventContents[0].PublishedAt)
	}
	byEvent, _, err := s.ListContents(ctx, ContentFilter{EventID: event.ID, Limit: 10})
	if err != nil || len(byEvent) != 2 {
		t.Fatalf("list event contents: %v (n=%d)", err, len(byEvent))
	}

	facets, err := s.ContentFacets(ctx)
	if err != nil {
		t.Fatalf("facets: %v", err)
	}
	if len(facets["platforms"]) == 0 || len(facets["content_types"]) == 0 || len(facets["source_types"]) == 0 {
		t.Fatalf("facets incomplete: %+v", facets)
	}
}
