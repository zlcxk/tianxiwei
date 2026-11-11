package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"xiweispace/backend/internal/models"
)

const (
	// editorialPickAdminLimit keeps the admin list bounded. The console does not
	// paginate picks, so an unbounded list would grow with every curated item.
	editorialPickAdminLimit = 200
	// editorialPickPublicLimit caps the home-page pick payload.
	editorialPickPublicLimit = 8
)

const editorialPickSelect = `
	SELECT p.id, p.content_id, p.title, p.description, p.cover_url, p.sort_order,
	       p.starts_at, p.ends_at, p.enabled, p.created_at, p.updated_at
	FROM editorial_picks p`

func scanEditorialPick(row rowScanner) (models.EditorialPick, error) {
	var p models.EditorialPick
	err := row.Scan(&p.ID, &p.ContentID, &p.Title, &p.Description, &p.CoverURL,
		&p.SortOrder, &p.StartsAt, &p.EndsAt, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	return p, mapNoRows(err)
}

func scanEditorialPicks(rows pgx.Rows) ([]models.EditorialPick, error) {
	out := make([]models.EditorialPick, 0)
	for rows.Next() {
		p, err := scanEditorialPick(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListEditorialPicks returns picks for the admin console. When includeInactive
// is false only enabled picks inside their display window are returned.
func (s *Store) ListEditorialPicks(ctx context.Context, includeInactive bool) ([]models.EditorialPick, error) {
	query := editorialPickSelect
	if !includeInactive {
		query += `
			WHERE p.enabled
			  AND (p.starts_at IS NULL OR p.starts_at <= now())
			  AND (p.ends_at IS NULL OR p.ends_at > now())`
	}
	query += fmt.Sprintf(` ORDER BY p.sort_order ASC, p.created_at DESC LIMIT %d`, editorialPickAdminLimit)
	rows, err := s.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEditorialPicks(rows)
}

// ActiveEditorialPicks returns enabled picks whose referenced content is still
// published and whose display window contains now.
func (s *Store) ActiveEditorialPicks(ctx context.Context, limit int) ([]models.EditorialPick, error) {
	if limit <= 0 || limit > editorialPickPublicLimit {
		limit = editorialPickPublicLimit
	}
	rows, err := s.Pool.Query(ctx, editorialPickSelect+`
		JOIN contents c ON c.id = p.content_id
		WHERE p.enabled
		  AND c.content_status = 'published'
		  AND (p.starts_at IS NULL OR p.starts_at <= now())
		  AND (p.ends_at IS NULL OR p.ends_at > now())
		ORDER BY p.sort_order ASC, p.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEditorialPicks(rows)
}

func (s *Store) GetEditorialPick(ctx context.Context, id string) (models.EditorialPick, error) {
	return scanEditorialPick(s.Pool.QueryRow(ctx, editorialPickSelect+` WHERE p.id=$1::uuid`, id))
}

type EditorialPickInput struct {
	ContentID   string     `json:"content_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CoverURL    string     `json:"cover_url"`
	SortOrder   int        `json:"sort_order"`
	StartsAt    *time.Time `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
	Enabled     *bool      `json:"enabled"`
}

func (s *Store) CreateEditorialPick(ctx context.Context, in EditorialPickInput) (models.EditorialPick, error) {
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO editorial_picks (content_id, title, description, cover_url, sort_order, starts_at, ends_at, enabled)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		in.ContentID, strings.TrimSpace(in.Title), strings.TrimSpace(in.Description),
		strings.TrimSpace(in.CoverURL), in.SortOrder, in.StartsAt, in.EndsAt, enabled,
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "editorial_picks_content_uidx") {
			return models.EditorialPick{}, ErrConflict
		}
		return models.EditorialPick{}, err
	}
	return s.GetEditorialPick(ctx, id)
}

func (s *Store) UpdateEditorialPick(ctx context.Context, id string, in EditorialPickInput) (models.EditorialPick, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE editorial_picks SET content_id=$2::uuid, title=$3, description=$4, cover_url=$5,
			sort_order=$6, starts_at=$7, ends_at=$8,
			enabled=COALESCE($9, enabled), updated_at=now()
		WHERE id=$1::uuid`,
		id, in.ContentID, strings.TrimSpace(in.Title), strings.TrimSpace(in.Description),
		strings.TrimSpace(in.CoverURL), in.SortOrder, in.StartsAt, in.EndsAt, in.Enabled,
	)
	if err != nil {
		if strings.Contains(err.Error(), "editorial_picks_content_uidx") {
			return models.EditorialPick{}, ErrConflict
		}
		return models.EditorialPick{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.EditorialPick{}, ErrNotFound
	}
	return s.GetEditorialPick(ctx, id)
}

func (s *Store) DeleteEditorialPick(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM editorial_picks WHERE id=$1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AttachEditorialPickContent loads the referenced content for all picks in a
// single query and marks favorites for the current user. raw_data is never
// loaded: the editorial console only needs public display fields. Content fetch
// failures leave Content nil so the API still returns the pick record for
// editorial cleanup. Public callers only see published content.
func (s *Store) AttachEditorialPickContent(ctx context.Context, picks []models.EditorialPick, userID string, admin bool) {
	if len(picks) == 0 {
		return
	}
	ids := make([]string, 0, len(picks))
	seen := make(map[string]struct{}, len(picks))
	for i := range picks {
		id := picks[i].ContentID
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}
	where := ` WHERE c.id = ANY($1::uuid[])`
	if !admin {
		where += ` AND c.content_status = 'published'`
	}
	rows, err := s.Pool.Query(ctx, contentSelect(false)+where, ids)
	if err != nil {
		return
	}
	defer rows.Close()
	byID := make(map[string]models.Content, len(ids))
	contents := make([]models.Content, 0, len(ids))
	for rows.Next() {
		c, err := scanContentRow(rows, false)
		if err != nil {
			continue
		}
		byID[c.ID] = c
		contents = append(contents, c)
	}
	if userID != "" && len(contents) > 0 {
		_ = s.markContentFavorites(ctx, userID, contents)
	}
	for i := range picks {
		if c, ok := byID[picks[i].ContentID]; ok {
			c := c
			picks[i].Content = &c
		}
	}
}
