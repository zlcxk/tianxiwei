package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"xiweispace/backend/internal/db"
	"xiweispace/backend/internal/models"
)

// TestEditorialPicksIntegration covers the pick lifecycle against a real
// database: published-only selection, ordering, active-window filtering and
// one-pick-per-content uniqueness. It is skipped unless TEST_DATABASE_URL is
// set to a disposable PostgreSQL database.
func TestEditorialPicksIntegration(t *testing.T) {
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

	published, _, err := s.ListContents(ctx, ContentFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list published contents: %v", err)
	}
	if len(published) == 0 {
		t.Fatal("seed has no published content to curate")
	}
	content := published[0]
	if _, err := pool.Exec(ctx, `DELETE FROM editorial_picks WHERE content_id=$1::uuid`, content.ID); err != nil {
		t.Fatalf("cleanup picks: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM editorial_picks WHERE content_id=$1::uuid`, content.ID)
	})

	enabled := true
	pick, err := s.CreateEditorialPick(ctx, EditorialPickInput{
		ContentID:   content.ID,
		Title:       "本周精选",
		Description: "编辑手记",
		SortOrder:   3,
		Enabled:     &enabled,
	})
	if err != nil {
		t.Fatalf("create pick: %v", err)
	}
	if pick.Title != "本周精选" || pick.SortOrder != 3 || !pick.Enabled {
		t.Fatalf("created pick = %+v", pick)
	}

	if _, err := s.CreateEditorialPick(ctx, EditorialPickInput{ContentID: content.ID}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate pick error = %v, want ErrConflict", err)
	}

	adminItems, err := s.ListEditorialPicks(ctx, true)
	if err != nil {
		t.Fatalf("list admin picks: %v", err)
	}
	s.AttachEditorialPickContent(ctx, adminItems, "", true)
	found := false
	for _, item := range adminItems {
		if item.ID == pick.ID {
			found = true
			if item.Content == nil || item.Content.ID != content.ID {
				t.Fatalf("pick content not attached: %+v", item.Content)
			}
		}
	}
	if !found {
		t.Fatal("created pick missing from admin list")
	}

	active, err := s.ActiveEditorialPicks(ctx, 12)
	if err != nil {
		t.Fatalf("active picks: %v", err)
	}
	if !containsPickID(active, pick.ID) {
		t.Fatal("enabled pick missing from active list")
	}

	home, err := s.Home(ctx, "")
	if err != nil {
		t.Fatalf("home with picks: %v", err)
	}
	found = false
	for _, item := range home.Picks {
		if item.ID != pick.ID {
			continue
		}
		found = true
		if item.Content == nil {
			t.Fatal("home pick has no public content")
		}
		if len(item.Content.RawData) != 0 {
			t.Fatal("home pick leaked raw_data")
		}
	}
	if !found {
		t.Fatal("enabled pick missing from home")
	}

	past := time.Now().Add(-2 * time.Hour)
	future := time.Now().Add(-1 * time.Hour)
	updated, err := s.UpdateEditorialPick(ctx, pick.ID, EditorialPickInput{
		ContentID: content.ID,
		Title:     "已结束的精选",
		StartsAt:  &past,
		EndsAt:    &future,
	})
	if err != nil {
		t.Fatalf("update pick window: %v", err)
	}
	if updated.EndsAt == nil {
		t.Fatal("updated pick lost ends_at")
	}
	active, err = s.ActiveEditorialPicks(ctx, 12)
	if err != nil {
		t.Fatalf("active picks after window: %v", err)
	}
	if containsPickID(active, pick.ID) {
		t.Fatal("expired pick still appears in active list")
	}

	disabled := false
	if _, err := s.UpdateEditorialPick(ctx, pick.ID, EditorialPickInput{
		ContentID: content.ID,
		Enabled:   &disabled,
	}); err != nil {
		t.Fatalf("disable pick: %v", err)
	}
	if err := s.DeleteEditorialPick(ctx, pick.ID); err != nil {
		t.Fatalf("delete pick: %v", err)
	}
	if _, err := s.GetEditorialPick(ctx, pick.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted pick error = %v, want ErrNotFound", err)
	}
}

func containsPickID(items []models.EditorialPick, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}
