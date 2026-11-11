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
	// Keep non-empty raw_data in the database so the assertions below prove the
	// editorial path never selects it.
	var originalRaw []byte
	if err := pool.QueryRow(ctx, `SELECT raw_data FROM contents WHERE id=$1::uuid`, content.ID).Scan(&originalRaw); err != nil {
		t.Fatalf("read raw_data: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE contents SET raw_data='{"integration":true}'::jsonb WHERE id=$1::uuid`, content.ID); err != nil {
		t.Fatalf("seed raw_data: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE contents SET raw_data=$2::jsonb WHERE id=$1::uuid`, content.ID, string(originalRaw))
	})
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
			if len(item.Content.RawData) != 0 {
				t.Fatal("admin pick must not load raw_data")
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
	if len(home.ForYou) != 0 {
		t.Fatalf("anonymous home for_you = %d items, want 0 (must not duplicate latest)", len(home.ForYou))
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

// TestEditorialPickBatchLimitIntegration covers the batched content lookup:
// the public list stays capped at 8, raw_data is never attached, and
// unpublished content is skipped for public callers.
func TestEditorialPickBatchLimitIntegration(t *testing.T) {
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

	contents, _, err := s.ListContents(ctx, ContentFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list published contents: %v", err)
	}
	if len(contents) < editorialPickPublicLimit+1 {
		t.Skipf("need %d published contents, have %d", editorialPickPublicLimit+1, len(contents))
	}
	contents = contents[:editorialPickPublicLimit+1]
	ids := make([]string, len(contents))
	for i := range contents {
		ids[i] = contents[i].ID
	}
	if _, err := pool.Exec(ctx, `DELETE FROM editorial_picks WHERE content_id = ANY($1::uuid[])`, ids); err != nil {
		t.Fatalf("cleanup picks: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM editorial_picks WHERE content_id = ANY($1::uuid[])`, ids)
	})

	rawBackup := make(map[string]string, len(ids))
	for _, id := range ids {
		var raw []byte
		if err := pool.QueryRow(ctx, `SELECT raw_data FROM contents WHERE id=$1::uuid`, id).Scan(&raw); err != nil {
			t.Fatalf("read raw_data: %v", err)
		}
		rawBackup[id] = string(raw)
	}
	if _, err := pool.Exec(ctx, `UPDATE contents SET raw_data='{"integration":true}'::jsonb WHERE id = ANY($1::uuid[])`, ids); err != nil {
		t.Fatalf("seed raw_data: %v", err)
	}
	t.Cleanup(func() {
		for id, raw := range rawBackup {
			_, _ = pool.Exec(context.Background(), `UPDATE contents SET raw_data=$2::jsonb WHERE id=$1::uuid`, id, raw)
		}
	})

	enabled := true
	for i, id := range ids {
		if _, err := s.CreateEditorialPick(ctx, EditorialPickInput{ContentID: id, SortOrder: i, Enabled: &enabled}); err != nil {
			t.Fatalf("create pick %d: %v", i, err)
		}
	}

	active, err := s.ActiveEditorialPicks(ctx, 12)
	if err != nil {
		t.Fatalf("active picks: %v", err)
	}
	if len(active) != editorialPickPublicLimit {
		t.Fatalf("active picks = %d, want public limit %d", len(active), editorialPickPublicLimit)
	}

	all, err := s.ListEditorialPicks(ctx, true)
	if err != nil {
		t.Fatalf("list admin picks: %v", err)
	}
	ours := make([]models.EditorialPick, 0, len(ids))
	for _, p := range all {
		for _, id := range ids {
			if p.ContentID == id {
				ours = append(ours, p)
				break
			}
		}
	}
	if len(ours) != len(ids) {
		t.Fatalf("admin picks for test contents = %d, want %d", len(ours), len(ids))
	}

	s.AttachEditorialPickContent(ctx, ours, "", true)
	for _, p := range ours {
		if p.Content == nil {
			t.Fatalf("admin attach missing content for %s", p.ContentID)
		}
		if len(p.Content.RawData) != 0 {
			t.Fatalf("admin attach loaded raw_data for %s", p.ContentID)
		}
	}

	var originalStatus string
	if err := pool.QueryRow(ctx, `SELECT content_status FROM contents WHERE id=$1::uuid`, ids[0]).Scan(&originalStatus); err != nil {
		t.Fatalf("read content status: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE contents SET content_status='pending' WHERE id=$1::uuid`, ids[0]); err != nil {
		t.Fatalf("hide content: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE contents SET content_status=$2 WHERE id=$1::uuid`, ids[0], originalStatus)
	})

	for i := range ours {
		ours[i].Content = nil
	}
	s.AttachEditorialPickContent(ctx, ours, "", false)
	for _, p := range ours {
		if p.ContentID == ids[0] {
			if p.Content != nil {
				t.Fatal("public attach must skip unpublished content")
			}
			continue
		}
		if p.Content == nil {
			t.Fatalf("public attach missing published content for %s", p.ContentID)
		}
		if len(p.Content.RawData) != 0 {
			t.Fatalf("public attach loaded raw_data for %s", p.ContentID)
		}
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
