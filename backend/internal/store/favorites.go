package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"xiweispace/backend/internal/models"
)

func (s *Store) AddFavorite(ctx context.Context, userID, targetType, targetID string) (string, error) {
	var query string
	switch targetType {
	case "content":
		query = `
			INSERT INTO favorites (user_id, target_type, target_id, snapshot)
			SELECT $1::uuid, 'content', c.id, jsonb_build_object(
				'title', c.title, 'subtitle', COALESCE(s.name, '站内资料'),
				'source_url', c.source_url, 'cover', c.cover_url
			)
			FROM contents c LEFT JOIN sources s ON s.id=c.source_id
			WHERE c.id=$2::uuid AND c.content_status='published'
			ON CONFLICT (user_id, target_type, target_id) DO NOTHING
			RETURNING id`
	case "work":
		query = `
			INSERT INTO favorites (user_id, target_type, target_id, snapshot)
			SELECT $1::uuid, 'work', w.id, jsonb_build_object(
				'title', w.title, 'subtitle', w.platform,
				'source_url', w.official_url, 'cover', w.cover
			)
			FROM works w WHERE w.id=$2::uuid
			ON CONFLICT (user_id, target_type, target_id) DO NOTHING
			RETURNING id`
	case "character":
		query = `
			INSERT INTO favorites (user_id, target_type, target_id, snapshot)
			SELECT $1::uuid, 'character', ch.id, jsonb_build_object(
				'title', ch.name, 'subtitle', w.title,
				'source_url', w.official_url, 'cover', ch.avatar
			)
			FROM characters ch JOIN works w ON w.id=ch.work_id
			WHERE ch.id=$2::uuid
			ON CONFLICT (user_id, target_type, target_id) DO NOTHING
			RETURNING id`
	default:
		return "", fmt.Errorf("invalid target type")
	}

	var id string
	err := s.Pool.QueryRow(ctx, query, userID, targetID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = s.Pool.QueryRow(ctx, `
		SELECT id FROM favorites WHERE user_id=$1::uuid AND target_type=$2 AND target_id=$3::uuid`,
		userID, targetType, targetID).Scan(&id)
	if err != nil {
		return "", ErrNotFound
	}
	return id, nil
}

func (s *Store) RemoveFavorite(ctx context.Context, userID, favoriteID string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM favorites WHERE id=$1::uuid AND user_id=$2::uuid`, favoriteID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RemoveFavoriteTarget(ctx context.Context, userID, targetType, targetID string) error {
	tag, err := s.Pool.Exec(ctx, `
		DELETE FROM favorites WHERE user_id=$1::uuid AND target_type=$2 AND target_id=$3::uuid`,
		userID, targetType, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) IsFavorited(ctx context.Context, userID, targetType, targetID string) bool {
	if userID == "" {
		return false
	}
	var exists bool
	_ = s.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM favorites WHERE user_id=$1::uuid AND target_type=$2 AND target_id=$3::uuid)`,
		userID, targetType, targetID).Scan(&exists)
	return exists
}

func (s *Store) ListFavorites(ctx context.Context, userID, targetType string) ([]models.FavoriteItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT f.id, f.target_type, f.target_id, f.created_at, f.snapshot,
			COALESCE(c.title, w.title, ch.name, f.snapshot->>'title', '已失效内容'),
			COALESCE(s.name, NULLIF(w.platform,''), cw.title, f.snapshot->>'subtitle', ''),
			COALESCE(c.source_url, w.official_url, f.snapshot->>'source_url', ''),
			COALESCE(c.cover_url, w.cover, f.snapshot->>'cover', ''),
			CASE
				WHEN f.target_type='content' THEN (c.id IS NULL OR c.content_status <> 'published')
				WHEN f.target_type='work' THEN w.id IS NULL
				ELSE ch.id IS NULL
			END
		FROM favorites f
		LEFT JOIN contents c ON f.target_type='content' AND c.id=f.target_id
		LEFT JOIN sources s ON s.id=c.source_id
		LEFT JOIN works w ON f.target_type='work' AND w.id=f.target_id
		LEFT JOIN characters ch ON f.target_type='character' AND ch.id=f.target_id
		LEFT JOIN works cw ON cw.id=ch.work_id
		WHERE f.user_id=$1::uuid AND ($2='' OR f.target_type=$2)
		ORDER BY f.created_at DESC`,
		userID, targetType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.FavoriteItem, 0)
	for rows.Next() {
		var f models.FavoriteItem
		var snapshot []byte
		if err := rows.Scan(&f.ID, &f.TargetType, &f.TargetID, &f.CreatedAt, &snapshot,
			&f.Title, &f.Subtitle, &f.SourceURL, &f.Cover, &f.Unavailable); err != nil {
			return nil, err
		}
		f.Snapshot = map[string]any{}
		_ = json.Unmarshal(snapshot, &f.Snapshot)
		out = append(out, f)
	}
	return out, rows.Err()
}
