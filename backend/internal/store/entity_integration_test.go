package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"xiweispace/backend/internal/db"
	"xiweispace/backend/internal/models"
)

// TestEntityAndRelevanceIntegration covers the person-entity config roundtrip
// and the relevance/filter fields on the search-candidate insert and admin
// content filters. It is skipped unless TEST_DATABASE_URL points at a
// disposable PostgreSQL database.
func TestEntityAndRelevanceIntegration(t *testing.T) {
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

	entity, err := s.GetArtistEntity(ctx)
	if err != nil {
		t.Fatalf("get artist entity: %v", err)
	}
	if entity.Name == "" {
		t.Fatal("seeded artist entity has no name")
	}

	saved, err := s.PutArtistEntity(ctx, models.ArtistEntity{
		Name:             entity.Name,
		Aliases:          []string{"集成测试别名"},
		VerifiedAccounts: []string{"集成测试工作室"},
		ExcludedTerms:    []string{"集成测试排除词"},
	})
	if err != nil {
		t.Fatalf("put artist entity: %v", err)
	}
	if len(saved.Aliases) != 1 || saved.Aliases[0] != "集成测试别名" {
		t.Fatalf("aliases = %v", saved.Aliases)
	}
	reloaded, err := s.GetArtistEntity(ctx)
	if err != nil {
		t.Fatalf("reload artist entity: %v", err)
	}
	if len(reloaded.VerifiedAccounts) != 1 || reloaded.VerifiedAccounts[0] != "集成测试工作室" {
		t.Fatalf("verified accounts = %v", reloaded.VerifiedAccounts)
	}
	if len(reloaded.ExcludedTerms) != 1 || reloaded.ExcludedTerms[0] != "集成测试排除词" {
		t.Fatalf("excluded terms = %v", reloaded.ExcludedTerms)
	}

	entityCtx, err := s.LoadEntityContext(ctx)
	if err != nil {
		t.Fatalf("load entity context: %v", err)
	}
	if entityCtx.Entity.Name != entity.Name {
		t.Fatalf("entity context name = %q", entityCtx.Entity.Name)
	}

	sources, err := s.ListSources(ctx)
	if err != nil {
		t.Fatalf("list sources with quality metrics: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("seeded sources missing")
	}
	if _, err := s.GetSource(ctx, sources[0].ID); err != nil {
		t.Fatalf("get source with quality metrics: %v", err)
	}

	var taskID string
	if err := pool.QueryRow(ctx, `SELECT id FROM search_tasks ORDER BY created_at LIMIT 1`).Scan(&taskID); err != nil {
		t.Fatalf("seeded search task: %v", err)
	}

	prefix := fmt.Sprintf("relevance-test-%d", time.Now().UnixNano())
	filtered, err := s.InsertDiscovered(ctx, DiscoveredContent{
		SearchTaskID:       taskID,
		Title:              prefix + " 田姓氏的来源",
		Summary:            "田姓，中华姓氏之一。",
		ContentType:        "other",
		URL:                "https://example.com/" + prefix + "/filtered",
		URLHash:            prefix + "-filtered",
		ContentHash:        prefix + "-filtered-content",
		PublishedPrecision: "unknown",
		OriginName:         "测试来源",
		OriginType:         "reference",
		MetadataStatus:     "ok",
		TrustLevel:         "reference",
		ReviewStatus:       "rejected",
		ContentStatus:      "rejected",
		ReviewReason:       "自动过滤：姓氏内容",
		RelevanceScore:     0,
		RelevanceLabel:     "filtered",
		FilterReason:       "dictionary_content",
		RawData:            []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("insert filtered content: %v", err)
	}
	if filtered != Inserted {
		t.Fatalf("filtered insert status = %q", filtered)
	}

	high, err := s.InsertDiscovered(ctx, DiscoveredContent{
		SearchTaskID:       taskID,
		Title:              prefix + " 田曦薇新剧开机",
		Summary:            "演员田曦薇出席开机仪式。",
		ContentType:        "news",
		URL:                "https://example.com/" + prefix + "/high",
		URLHash:            prefix + "-high",
		ContentHash:        prefix + "-high-content",
		PublishedPrecision: "unknown",
		OriginName:         "测试媒体",
		OriginType:         "media",
		MetadataStatus:     "ok",
		TrustLevel:         "media",
		ReviewStatus:       "pending",
		ContentStatus:      "pending",
		ReviewReason:       "相关度 80 分",
		RelevanceScore:     80,
		RelevanceLabel:     "high",
		RawData:            []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("insert high content: %v", err)
	}
	if high != Inserted {
		t.Fatalf("high insert status = %q", high)
	}

	filteredList, _, err := s.ListContents(ctx, ContentFilter{
		FilterReason:       "dictionary_content",
		IncludeUnpublished: true,
		Limit:              10,
	})
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if !containsTitle(filteredList, prefix+" 田姓氏的来源") {
		t.Fatalf("filtered content missing from filter_reason query (%d items)", len(filteredList))
	}
	if containsTitle(filteredList, prefix+" 田曦薇新剧开机") {
		t.Fatal("high-relevance content appeared in filter_reason query")
	}

	highList, _, err := s.ListContents(ctx, ContentFilter{
		RelevanceLabel:     "high",
		IncludeUnpublished: true,
		Limit:              10,
	})
	if err != nil {
		t.Fatalf("list high: %v", err)
	}
	if !containsTitle(highList, prefix+" 田曦薇新剧开机") {
		t.Fatalf("high content missing from relevance query (%d items)", len(highList))
	}
	for _, item := range highList {
		if item.RelevanceLabel != "high" || item.RelevanceScore != 80 {
			t.Fatalf("scanned relevance fields = %q/%d", item.RelevanceLabel, item.RelevanceScore)
		}
	}
}

func containsTitle(items []models.Content, title string) bool {
	for _, item := range items {
		if item.Title == title {
			return true
		}
	}
	return false
}
