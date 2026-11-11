package store

import (
	"context"
	"math"

	"xiweispace/backend/internal/models"
)

// dataHealthLimit caps the admin missing-item list. The console pages through
// filters rather than loading every row, so an unbounded list would grow with
// the content library.
const dataHealthLimit = 200

// dataHealthItems builds one row per editorial entity with the labels of the
// fields it is missing. Counts and the filterable list both read from this CTE,
// so the panel totals always match the rows shown below them.
//
// Only published contents and events are checked: drafts and pending candidates
// are work in progress and would drown out real gaps in public data.
const dataHealthItems = `
WITH items AS (
    SELECT 'artist'::text AS entity_type, a.id, a.name AS title, '艺人'::text AS category,
           a.avatar AS preview, a.updated_at,
           ARRAY_REMOVE(ARRAY[
               CASE WHEN COALESCE(a.avatar, '') = '' THEN '头像' END,
               CASE WHEN COALESCE(a.cover, '') = '' THEN '封面' END
           ], NULL) AS missing
    FROM artists a
    UNION ALL
    SELECT 'work', w.id, w.title, '作品', w.cover, w.updated_at,
           ARRAY_REMOVE(ARRAY[
               CASE WHEN COALESCE(w.cover, '') = '' THEN '封面' END,
               CASE WHEN COALESCE(w.description, '') = '' THEN '简介' END,
               CASE WHEN w.release_date IS NULL THEN '日期' END
           ], NULL)
    FROM works w
    UNION ALL
    SELECT 'character', c.id, c.name, '角色', c.avatar, c.created_at,
           ARRAY_REMOVE(ARRAY[
               CASE WHEN COALESCE(c.avatar, '') = '' THEN '头像' END,
               CASE WHEN COALESCE(c.description, '') = '' THEN '简介' END
           ], NULL)
    FROM characters c
    UNION ALL
    SELECT 'content', c.id, c.title, '内容', c.cover_url, c.updated_at,
           ARRAY_REMOVE(ARRAY[
               CASE WHEN COALESCE(c.cover_url, '') = '' THEN '封面' END,
               CASE WHEN COALESCE(c.summary, '') = '' THEN '简介' END,
               CASE WHEN c.published_at IS NULL THEN '日期' END,
               CASE WHEN NOT EXISTS (SELECT 1 FROM content_works cw WHERE cw.content_id = c.id)
                     AND NOT EXISTS (SELECT 1 FROM content_characters cc WHERE cc.content_id = c.id)
                     AND NOT EXISTS (SELECT 1 FROM content_events ce WHERE ce.content_id = c.id)
                    THEN '关联内容' END,
               CASE WHEN COALESCE(c.source_url, '') = '' THEN '来源链接' END
           ], NULL)
    FROM contents c
    WHERE c.content_status = 'published'
    UNION ALL
    SELECT 'event', e.id, e.title, '事件', e.cover_url, e.updated_at,
           ARRAY_REMOVE(ARRAY[
               CASE WHEN COALESCE(e.cover_url, '') = '' THEN '封面' END,
               CASE WHEN COALESCE(e.summary, '') = '' THEN '简介' END,
               CASE WHEN e.event_date IS NULL THEN '日期' END,
               CASE WHEN NOT EXISTS (SELECT 1 FROM content_events ce WHERE ce.event_id = e.id)
                    THEN '关联内容' END
           ], NULL)
    FROM events e
    WHERE e.status = 'published'
)
`

// dataHealthFilterMissing maps the UI filters to the field labels stored in
// items.missing. An unknown or "all" filter returns no restriction.
var dataHealthFilterMissing = map[string][]string{
	"image":      {"头像", "封面"},
	"summary":    {"简介"},
	"date":       {"日期"},
	"relation":   {"关联内容"},
	"source_url": {"来源链接"},
}

// DataHealth returns completeness counts, image coverage and the missing-field
// list for the admin console.
func (s *Store) DataHealth(ctx context.Context, filter string, limit int) (models.DataHealth, error) {
	if limit <= 0 || limit > dataHealthLimit {
		limit = dataHealthLimit
	}
	labels := dataHealthFilterMissing[filter]
	if labels == nil {
		labels = []string{}
	}

	out := models.DataHealth{Filter: filter}
	if filter == "" {
		out.Filter = "all"
	}
	var c models.DataHealthCounts
	err := s.Pool.QueryRow(ctx, dataHealthItems+`
		SELECT
		    (SELECT count(*) FROM artists),
		    (SELECT count(*) FROM artists WHERE COALESCE(avatar, '') <> ''),
		    (SELECT count(*) FROM artists WHERE COALESCE(cover, '') <> ''),
		    (SELECT count(*) FROM works),
		    (SELECT count(*) FROM works WHERE COALESCE(cover, '') <> ''),
		    (SELECT count(*) FROM characters),
		    (SELECT count(*) FROM characters WHERE COALESCE(avatar, '') <> ''),
		    (SELECT count(*) FROM contents WHERE content_status = 'published'),
		    (SELECT count(*) FROM contents WHERE content_status = 'published' AND COALESCE(cover_url, '') <> ''),
		    (SELECT count(*) FROM events WHERE status = 'published'),
		    (SELECT count(*) FROM events WHERE status = 'published' AND COALESCE(cover_url, '') <> ''),
		    (SELECT count(*) FROM items WHERE missing && ARRAY['头像', '封面']::text[]),
		    (SELECT count(*) FROM items WHERE '简介' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE '日期' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE '关联内容' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE '来源链接' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE entity_type = 'artist' AND '头像' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE entity_type = 'artist' AND '封面' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE entity_type = 'work' AND '封面' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE entity_type = 'character' AND '头像' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE entity_type = 'content' AND '封面' = ANY(missing)),
		    (SELECT count(*) FROM items WHERE entity_type = 'event' AND '封面' = ANY(missing))
	`).Scan(
		&c.Artists, &c.ArtistAvatars, &c.ArtistCovers,
		&c.Works, &c.WorkCovers,
		&c.Characters, &c.CharacterAvatars,
		&c.PublishedContents, &c.ContentCovers,
		&c.PublishedEvents, &c.EventCovers,
		&c.MissingImages, &c.MissingSummaries, &c.MissingDates,
		&c.MissingRelations, &c.MissingSourceURLs,
		&c.MissingArtistAvatar, &c.MissingArtistCover, &c.MissingWorkCover,
		&c.MissingCharacterAvatar, &c.MissingContentCover, &c.MissingEventCover,
	)
	if err != nil {
		return models.DataHealth{}, err
	}
	out.Counts = c
	out.Coverage = models.DataHealthCoverage{
		ArtistAvatar:    coveragePercent(c.ArtistAvatars, c.Artists),
		ArtistCover:     coveragePercent(c.ArtistCovers, c.Artists),
		WorkCover:       coveragePercent(c.WorkCovers, c.Works),
		CharacterAvatar: coveragePercent(c.CharacterAvatars, c.Characters),
		ContentCover:    coveragePercent(c.ContentCovers, c.PublishedContents),
		EventCover:      coveragePercent(c.EventCovers, c.PublishedEvents),
	}

	rows, err := s.Pool.Query(ctx, dataHealthItems+`
		SELECT entity_type, id, title, category, preview, missing, updated_at,
		       count(*) OVER() AS total
		FROM items
		WHERE cardinality($1::text[]) = 0 OR missing && $1::text[]
		ORDER BY CASE entity_type
		             WHEN 'content' THEN 0
		             WHEN 'event' THEN 1
		             WHEN 'work' THEN 2
		             WHEN 'character' THEN 3
		             ELSE 4
		         END,
		         updated_at DESC NULLS LAST, title
		LIMIT $2`, labels, limit)
	if err != nil {
		return models.DataHealth{}, err
	}
	defer rows.Close()

	out.Items = make([]models.DataHealthItem, 0)
	for rows.Next() {
		var item models.DataHealthItem
		if err := rows.Scan(&item.Type, &item.ID, &item.Title, &item.Category,
			&item.Preview, &item.Missing, &item.UpdatedAt, &out.Total); err != nil {
			return models.DataHealth{}, err
		}
		item.Missing = append([]string{}, item.Missing...)
		item.EditPath = dataHealthEditPath(item.Type, item.ID)
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return models.DataHealth{}, err
	}
	out.Returned = len(out.Items)
	return out, nil
}

func coveragePercent(covered, total int) float64 {
	if total <= 0 {
		return 100
	}
	return math.Round(float64(covered)/float64(total)*1000) / 10
}

// dataHealthEditPath links a missing row to the editor that owns the field.
// Artists have no row-level editor, so they open the artist tab.
func dataHealthEditPath(entityType, id string) string {
	switch entityType {
	case "artist":
		return "/admin/base?tab=artist"
	case "work":
		return "/admin/base?tab=works&edit=" + id
	case "character":
		return "/admin/base?tab=characters&edit=" + id
	case "content":
		return "/admin/contents?edit=" + id
	case "event":
		return "/admin/events?edit=" + id
	default:
		return "/admin"
	}
}
