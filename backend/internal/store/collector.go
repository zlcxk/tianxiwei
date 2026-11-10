package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type CollectedContent struct {
	SourceID           string
	SourceItemID       string
	Title              string
	Summary            string
	ContentType        string
	Author             string
	URL                string
	CoverURL           string
	URLHash            string
	ContentHash        string
	PublishedAt        *time.Time
	PublishedPrecision string
	TrustLevel         string
	IsOfficial         bool
	ReviewStatus       string
	ContentStatus      string
	ReviewReason       string
	RawData            []byte
	SceneIDs           []string
}

const (
	Inserted  = "inserted"
	Duplicate = "duplicate"
)

// InsertCollected performs source-item, canonical URL and content-fingerprint
// deduplication before inserting a normalized item. It is safe to run
// concurrently because the database also enforces unique indexes.
func (s *Store) InsertCollected(ctx context.Context, c CollectedContent) (string, error) {
	status := ""
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var existing string
		err := tx.QueryRow(ctx, `
			SELECT id FROM contents WHERE source_id=$1::uuid AND source_item_id=$2 LIMIT 1`,
			c.SourceID, c.SourceItemID).Scan(&existing)
		if err == nil {
			status = Duplicate
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if c.URLHash != "" {
			err = tx.QueryRow(ctx, `SELECT id FROM contents WHERE url_hash=$1 LIMIT 1`, c.URLHash).Scan(&existing)
			if err == nil {
				status = Duplicate
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if c.ContentHash != "" {
			err = tx.QueryRow(ctx, `
				SELECT id FROM contents WHERE content_hash=$1 AND collected_at > now() - interval '14 days' LIMIT 1`,
				c.ContentHash).Scan(&existing)
			if err == nil {
				status = Duplicate
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		raw := c.RawData
		if len(raw) == 0 {
			raw = []byte("{}")
		}
		if c.PublishedPrecision == "" {
			if c.PublishedAt == nil {
				c.PublishedPrecision = "unknown"
			} else {
				c.PublishedPrecision = "verified"
			}
		}
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO contents (
				artist_id, source_id, source_item_id, title, summary, content_type, source_author,
				source_url, cover_url, published_at, discovered_at, published_precision, collected_at,
				trust_level, is_official, review_status, content_status, review_reason,
				url_hash, content_hash, raw_data
			)
			SELECT a.id, $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10, now(), $11, $12, $13, $14, $15,
			       NULLIF($16,''), NULLIF($17,''), $18::jsonb
			FROM artists a ORDER BY a.created_at LIMIT 1
			RETURNING id`,
			c.SourceID, c.SourceItemID, c.Title, c.Summary, c.ContentType, c.Author,
			c.URL, c.CoverURL, c.PublishedAt, c.PublishedPrecision, c.TrustLevel, c.IsOfficial,
			c.ReviewStatus, c.ContentStatus, c.ReviewReason, c.URLHash, c.ContentHash, raw,
		).Scan(&id); err != nil {
			// Unique violations can happen in a race with another worker.
			if strings.Contains(err.Error(), "contents_source_item_uidx") || strings.Contains(err.Error(), "contents_url_hash_uidx") {
				status = Duplicate
				return nil
			}
			return err
		}
		for _, sceneID := range c.SceneIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO content_scenes (content_id, scene_id) VALUES ($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, id, sceneID); err != nil {
				return err
			}
		}
		status = Inserted
		return nil
	})
	return status, err
}
