package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"xiweispace/backend/internal/models"
)

const artistEntityKey = "artist_entity"

// defaultExcludedTerms is used when the admin has not saved a custom entity
// config yet. It targets dictionary / surname pages that match the artist's
// surname but are not about the artist.
var defaultExcludedTerms = []string{
	"田姓", "田姓氏", "田字解释", "汉字田", "田字组词",
	"田的部首", "田的拼音", "田的笔画", "田字成语", "百家姓",
}

func normalizeEntity(e *models.ArtistEntity) {
	e.Name = strings.TrimSpace(e.Name)
	e.Aliases = cleanTerms(e.Aliases, 20)
	e.VerifiedAccounts = cleanTerms(e.VerifiedAccounts, 30)
	e.ExcludedTerms = cleanTerms(e.ExcludedTerms, 100)
}

func cleanTerms(items []string, max int) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
		if len(out) >= max {
			break
		}
	}
	return out
}

// GetArtistEntity returns the editorial person-entity config. A missing config
// falls back to the verified artist name plus the default excluded terms.
func (s *Store) GetArtistEntity(ctx context.Context) (models.ArtistEntity, error) {
	entity := models.ArtistEntity{}
	configured := false
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, artistEntityKey).Scan(&raw)
	switch {
	case err == nil:
		configured = true
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &entity); err != nil {
				return entity, fmt.Errorf("人物实体配置格式不正确: %w", err)
			}
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return entity, err
	}
	if !configured {
		entity.ExcludedTerms = append([]string(nil), defaultExcludedTerms...)
	}
	if strings.TrimSpace(entity.Name) == "" {
		var name string
		if err := s.Pool.QueryRow(ctx, `SELECT name FROM artists ORDER BY created_at LIMIT 1`).Scan(&name); err == nil {
			entity.Name = name
		}
	}
	normalizeEntity(&entity)
	return entity, nil
}

func (s *Store) PutArtistEntity(ctx context.Context, e models.ArtistEntity) (models.ArtistEntity, error) {
	normalizeEntity(&e)
	if e.Name == "" {
		return e, fmt.Errorf("人物姓名不能为空")
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ($1, $2::jsonb)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`,
		artistEntityKey, raw)
	if err != nil {
		return e, err
	}
	return e, nil
}

// EntityContext bundles the person entity with the verified works, characters
// and events used for the "+15 related" relevance signals.
type EntityContext struct {
	Entity         models.ArtistEntity
	WorkTitles     []string
	CharacterNames []string
	EventTitles    []string
}

func (s *Store) LoadEntityContext(ctx context.Context) (EntityContext, error) {
	entity, err := s.GetArtistEntity(ctx)
	if err != nil {
		return EntityContext{}, err
	}
	out := EntityContext{Entity: entity}
	artistID := ""
	_ = s.Pool.QueryRow(ctx, `SELECT id FROM artists ORDER BY created_at LIMIT 1`).Scan(&artistID)
	if artistID == "" {
		return out, nil
	}
	out.WorkTitles = s.stringColumn(ctx, `
		SELECT title FROM works WHERE artist_id=$1::uuid ORDER BY release_date DESC NULLS LAST`, artistID)
	out.CharacterNames = s.stringColumn(ctx, `
		SELECT ch.name FROM characters ch JOIN works w ON w.id=ch.work_id
		WHERE w.artist_id=$1::uuid ORDER BY ch.created_at`, artistID)
	out.EventTitles = s.stringColumn(ctx, `
		SELECT title FROM events WHERE artist_id=$1::uuid AND status='published'
		ORDER BY event_date DESC NULLS LAST`, artistID)
	return out, nil
}

func (s *Store) stringColumn(ctx context.Context, query string, args ...any) []string {
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return out
		}
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}
