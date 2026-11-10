package store

import (
	"context"
	"strings"

	"xiweispace/backend/internal/models"
)

const eventSelect = `
	SELECT e.id, e.title, e.summary, e.cover_url,
	       COALESCE(to_char(e.event_date, 'YYYY-MM-DD'), ''), e.location, e.status,
	       (SELECT count(*) FROM content_events ce JOIN contents c ON c.id=ce.content_id
	        WHERE ce.event_id=e.id AND c.content_status='published'),
	       e.created_at, e.updated_at
	FROM events e`

func scanEvent(row rowScanner) (models.Event, error) {
	var e models.Event
	err := row.Scan(&e.ID, &e.Title, &e.Summary, &e.CoverURL, &e.EventDate, &e.Location,
		&e.Status, &e.ContentCount, &e.CreatedAt, &e.UpdatedAt)
	return e, mapNoRows(err)
}

func (s *Store) ListEvents(ctx context.Context, includeDraft bool) ([]models.Event, error) {
	query := eventSelect
	if !includeDraft {
		query += ` WHERE e.status='published'`
	}
	query += ` ORDER BY e.event_date DESC NULLS LAST, e.created_at DESC`
	rows, err := s.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Event, 0)
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetEvent(ctx context.Context, id string) (models.Event, error) {
	return scanEvent(s.Pool.QueryRow(ctx, eventSelect+` WHERE e.id=$1::uuid`, id))
}

type EventInput struct {
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	CoverURL  string `json:"cover_url"`
	EventDate string `json:"event_date"`
	Location  string `json:"location"`
	Status    string `json:"status"`
}

func (s *Store) CreateEvent(ctx context.Context, artistID string, in EventInput) (models.Event, error) {
	if in.Status == "" {
		in.Status = "draft"
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO events (artist_id, title, summary, cover_url, event_date, location, status)
		VALUES ($1::uuid,$2,$3,$4,NULLIF($5,'')::date,$6,$7) RETURNING id`,
		artistID, in.Title, in.Summary, in.CoverURL, in.EventDate, in.Location, in.Status).Scan(&id)
	if err != nil {
		return models.Event{}, err
	}
	return s.GetEvent(ctx, id)
}

func (s *Store) UpdateEvent(ctx context.Context, id string, in EventInput) (models.Event, error) {
	status := in.Status
	if strings.TrimSpace(status) == "" {
		status = "draft"
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE events SET title=$2, summary=$3, cover_url=$4,
			event_date=NULLIF($5,'')::date, location=$6, status=$7, updated_at=now()
		WHERE id=$1::uuid`,
		id, in.Title, in.Summary, in.CoverURL, in.EventDate, in.Location, status)
	if err != nil {
		return models.Event{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Event{}, ErrNotFound
	}
	return s.GetEvent(ctx, id)
}

func (s *Store) DeleteEvent(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM events WHERE id=$1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) EventDetail(ctx context.Context, id, userID string) (models.Event, []models.Content, error) {
	event, err := s.GetEvent(ctx, id)
	if err != nil {
		return models.Event{}, nil, err
	}
	items, _, err := s.ListContents(ctx, ContentFilter{EventID: id, Limit: 100})
	if err != nil {
		return models.Event{}, nil, err
	}
	_ = s.markContentFavorites(ctx, userID, items)
	return event, items, nil
}
