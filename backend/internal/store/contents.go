package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"xiweispace/backend/internal/models"
)

const contentFields = `
	c.id, c.title, c.summary, c.content_type, c.source_id,
	COALESCE(NULLIF(c.origin_name, ''), s.name, '站内资料'),
	COALESCE(NULLIF(c.origin_platform, ''), s.platform, ''),
	COALESCE(NULLIF(c.origin_type, ''), s.source_type, 'internal'),
	c.source_author, c.source_url, c.cover_url,
	COALESCE(c.published_at, c.discovered_at), c.published_at, c.discovered_at,
	c.published_precision, c.collected_at, c.created_at,
	c.origin_name, c.origin_platform, c.origin_type,
	c.metadata_status, c.metadata_error, c.search_task_id,
	c.trust_level, c.is_official, c.review_status, c.content_status, c.review_reason,
	c.relevance_score, c.relevance_label, c.filter_reason,
	COALESCE((
		SELECT json_agg(json_build_object(
			'id', sc.id, 'code', sc.code, 'name', sc.name, 'description', sc.description,
			'icon', sc.icon, 'sort_order', sc.sort_order, 'enabled', sc.enabled
		) ORDER BY sc.sort_order)
		FROM content_scenes cs JOIN scenes sc ON sc.id = cs.scene_id WHERE cs.content_id = c.id
	), '[]'::json),
	COALESCE((
		SELECT json_agg(json_build_object(
			'id', w.id, 'title', w.title, 'type', w.type,
			'release_date', COALESCE(to_char(w.release_date, 'YYYY-MM-DD'), ''), 'cover', w.cover
		) ORDER BY w.release_date DESC NULLS LAST)
		FROM content_works cw JOIN works w ON w.id = cw.work_id WHERE cw.content_id = c.id
	), '[]'::json),
	COALESCE((
		SELECT json_agg(json_build_object(
			'id', ch.id, 'name', ch.name, 'work_id', ch.work_id, 'work_title', w.title
		) ORDER BY ch.created_at)
		FROM content_characters cc JOIN characters ch ON ch.id = cc.character_id
		JOIN works w ON w.id = ch.work_id WHERE cc.content_id = c.id
	), '[]'::json),
	COALESCE((
		SELECT json_agg(json_build_object(
			'id', e.id, 'title', e.title,
			'event_date', COALESCE(to_char(e.event_date, 'YYYY-MM-DD'), '')
		) ORDER BY e.event_date DESC NULLS LAST, e.created_at)
		FROM content_events ce JOIN events e ON e.id = ce.event_id WHERE ce.content_id = c.id
	), '[]'::json)`

const contentFrom = `
	FROM contents c LEFT JOIN sources s ON s.id = c.source_id`

func contentSelect(withRaw bool) string {
	raw := ""
	if withRaw {
		raw = `, COALESCE(c.raw_data, '{}'::jsonb)`
	}
	return `SELECT ` + contentFields + raw + contentFrom
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanContentRow(row rowScanner, withRaw bool) (models.Content, error) {
	var c models.Content
	var scenesJSON, worksJSON, charsJSON, eventsJSON []byte
	var publishedAt *time.Time
	dest := []any{
		&c.ID, &c.Title, &c.Summary, &c.ContentType, &c.SourceID,
		&c.SourceName, &c.SourcePlatform, &c.SourceType,
		&c.SourceAuthor, &c.SourceURL, &c.CoverURL,
		&c.EffectiveAt, &publishedAt, &c.DiscoveredAt,
		&c.PublishedPrecision, &c.CollectedAt, &c.CreatedAt,
		&c.OriginName, &c.OriginPlatform, &c.OriginType,
		&c.MetadataStatus, &c.MetadataError, &c.SearchTaskID,
		&c.TrustLevel, &c.IsOfficial, &c.ReviewStatus, &c.ContentStatus, &c.ReviewReason,
		&c.RelevanceScore, &c.RelevanceLabel, &c.FilterReason,
		&scenesJSON, &worksJSON, &charsJSON, &eventsJSON,
	}
	var raw []byte
	if withRaw {
		dest = append(dest, &raw)
	}
	if err := row.Scan(dest...); err != nil {
		return c, mapNoRows(err)
	}
	c.PublishedAt = publishedAt
	c.Scenes = []models.Scene{}
	c.Works = []models.WorkRef{}
	c.Characters = []models.CharacterRef{}
	c.Events = []models.EventRef{}
	_ = json.Unmarshal(scenesJSON, &c.Scenes)
	_ = json.Unmarshal(worksJSON, &c.Works)
	_ = json.Unmarshal(charsJSON, &c.Characters)
	_ = json.Unmarshal(eventsJSON, &c.Events)
	if withRaw {
		c.RawData = raw
	}
	return c, nil
}

type ContentCursor struct {
	EffectiveAt time.Time
	CreatedAt   time.Time
	ID          string
}

type ContentFilter struct {
	ArtistID           string
	Keyword            string
	Scenes             []string
	SourceType         string
	SourceQuery        string
	SourceID           string
	ContentType        string
	ContentTypes       []string
	Official           *bool
	WorkID             string
	CharacterID        string
	EventID            string
	StartTime          *time.Time
	EndTime            *time.Time
	IncludeUnpublished bool
	ContentStatus      string
	ReviewStatus       string
	OnlyCandidates     bool
	SearchTaskID       string
	MetadataStatus     string
	FilterReason       string
	RelevanceLabel     string
	Sort               string
	Cursor             *ContentCursor
	Limit              int
	Offset             int
}

func (f ContentFilter) where() (string, []any) {
	var clauses []string
	var args []any
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	effectiveName := `COALESCE(NULLIF(c.origin_name,''), s.name, '')`
	effectivePlatform := `COALESCE(NULLIF(c.origin_platform,''), s.platform, '')`
	if f.ArtistID != "" {
		add(`c.artist_id = $%d::uuid`, f.ArtistID)
	}
	if !f.IncludeUnpublished {
		clauses = append(clauses, `c.content_status = 'published'`)
	} else if f.ContentStatus != "" {
		add(`c.content_status = $%d`, f.ContentStatus)
	}
	if f.ReviewStatus != "" {
		add(`c.review_status = $%d`, f.ReviewStatus)
	}
	if f.Keyword != "" {
		idx := len(args) + 1
		args = append(args, "%"+f.Keyword+"%")
		clauses = append(clauses, fmt.Sprintf(
			`(c.title ILIKE $%d OR c.summary ILIKE $%d OR %s ILIKE $%d OR %s ILIKE $%d)`,
			idx, idx, effectiveName, idx, effectivePlatform, idx))
	}
	if len(f.Scenes) > 0 {
		add(`EXISTS (SELECT 1 FROM content_scenes cs JOIN scenes sc ON sc.id=cs.scene_id WHERE cs.content_id=c.id AND sc.code = ANY($%d::text[]))`, f.Scenes)
	}
	if f.SourceType != "" {
		add(`COALESCE(NULLIF(c.origin_type,''), s.source_type,'internal') = $%d`, f.SourceType)
	}
	if f.SourceQuery != "" {
		idx := len(args) + 1
		args = append(args, "%"+f.SourceQuery+"%")
		clauses = append(clauses, fmt.Sprintf(`(%s ILIKE $%d OR %s ILIKE $%d)`, effectiveName, idx, effectivePlatform, idx))
	}
	if f.SourceID != "" {
		add(`c.source_id = $%d::uuid`, f.SourceID)
	}
	if f.ContentType != "" {
		add(`c.content_type = $%d`, f.ContentType)
	}
	if len(f.ContentTypes) > 0 {
		add(`c.content_type = ANY($%d::text[])`, f.ContentTypes)
	}
	if f.Official != nil {
		add(`c.is_official = $%d`, *f.Official)
	}
	if f.WorkID != "" {
		add(`EXISTS (SELECT 1 FROM content_works cw WHERE cw.content_id=c.id AND cw.work_id=$%d::uuid)`, f.WorkID)
	}
	if f.CharacterID != "" {
		add(`EXISTS (SELECT 1 FROM content_characters cc WHERE cc.content_id=c.id AND cc.character_id=$%d::uuid)`, f.CharacterID)
	}
	if f.EventID != "" {
		add(`EXISTS (SELECT 1 FROM content_events ce WHERE ce.content_id=c.id AND ce.event_id=$%d::uuid)`, f.EventID)
	}
	if f.StartTime != nil {
		add(`COALESCE(c.published_at, c.discovered_at) >= $%d`, *f.StartTime)
	}
	if f.EndTime != nil {
		add(`COALESCE(c.published_at, c.discovered_at) <= $%d`, *f.EndTime)
	}
	if f.OnlyCandidates {
		clauses = append(clauses, `c.search_task_id IS NOT NULL`)
	}
	if f.SearchTaskID != "" {
		add(`c.search_task_id = $%d::uuid`, f.SearchTaskID)
	}
	if f.MetadataStatus != "" {
		add(`c.metadata_status = $%d`, f.MetadataStatus)
	}
	if f.FilterReason != "" {
		add(`c.filter_reason = $%d`, f.FilterReason)
	}
	if f.RelevanceLabel != "" {
		add(`c.relevance_label = $%d`, f.RelevanceLabel)
	}
	if f.Cursor != nil {
		idx := len(args) + 1
		args = append(args, f.Cursor.EffectiveAt, f.Cursor.CreatedAt, f.Cursor.ID)
		clauses = append(clauses, fmt.Sprintf(
			`(COALESCE(c.published_at, c.discovered_at), c.created_at, c.id) < ($%d, $%d, $%d::uuid)`,
			idx, idx+1, idx+2))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (f ContentFilter) order() string {
	switch f.Sort {
	case "oldest":
		return " ORDER BY COALESCE(c.published_at, c.discovered_at) ASC, c.created_at ASC, c.id ASC"
	case "trust":
		return ` ORDER BY CASE c.trust_level WHEN 'official' THEN 5 WHEN 'reference' THEN 4 WHEN 'media' THEN 3 WHEN 'internal' THEN 2 ELSE 1 END DESC, COALESCE(c.published_at, c.discovered_at) DESC, c.id DESC`
	default:
		return " ORDER BY COALESCE(c.published_at, c.discovered_at) DESC, c.created_at DESC, c.id DESC"
	}
}

func (s *Store) ListContents(ctx context.Context, f ContentFilter) ([]models.Content, int, error) {
	where, args := f.where()
	var total int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*)`+contentFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	args = append(args, f.Limit, f.Offset)
	query := contentSelect(false) + where + f.order() +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]models.Content, 0, f.Limit)
	for rows.Next() {
		c, err := scanContentRow(rows, false)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, c)
	}
	return items, total, rows.Err()
}

func (s *Store) ListRecommended(ctx context.Context, f ContentFilter, userID string) ([]models.Content, int, error) {
	pageSize := f.Limit
	offset := f.Offset
	f.Sort = "latest"
	f.Limit = 300
	f.Offset = 0
	items, _, err := s.ListContents(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	weights, _ := s.SceneWeights(ctx, userID)
	weightCfg := s.GetRecommendWeights(ctx)
	now := time.Now()
	for i := range items {
		items[i].Scenes = removeAllScene(items[i].Scenes)
	}
	score := func(c models.Content) float64 {
		scene := 0.0
		for _, sc := range c.Scenes {
			if w, ok := weights[sc.Code]; ok {
				v := float64(w) / 5
				if v > scene {
					scene = v
				}
			}
		}
		if w, ok := weights["all"]; ok {
			v := float64(w) / 5
			if v > scene {
				scene = v
			}
		}
		hours := now.Sub(c.EffectiveAt).Hours()
		if hours < 0 {
			hours = 0
		}
		recency := math.Exp(-hours / 240)
		trust := map[string]float64{"official": 1, "reference": 0.75, "media": 0.8, "internal": 0.65, "community": 0.4}[c.TrustLevel]
		// Popularity stays at 0 until a verifiable heat signal is available.
		return scene*float64(weightCfg.Scene)/100 +
			recency*float64(weightCfg.Recency)/100 +
			trust*float64(weightCfg.Trust)/100
	}
	sortContents(items, score)
	total := len(items)
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	start := offset
	if start < 0 {
		start = 0
	}
	if start >= total {
		return []models.Content{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return items[start:end], total, nil
}

func removeAllScene(scenes []models.Scene) []models.Scene {
	out := scenes[:0]
	for _, sc := range scenes {
		if sc.Code != "all" {
			out = append(out, sc)
		}
	}
	return out
}

func sortContents(items []models.Content, score func(models.Content) float64) {
	scores := make(map[string]float64, len(items))
	for _, c := range items {
		scores[c.ID] = score(c)
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && scores[items[j].ID] > scores[items[j-1].ID]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func (s *Store) GetContent(ctx context.Context, id string) (models.Content, error) {
	return scanContentRow(s.Pool.QueryRow(ctx, contentSelect(false)+` WHERE c.id=$1::uuid`, id), false)
}

func (s *Store) GetContentAdmin(ctx context.Context, id string) (models.Content, error) {
	return scanContentRow(s.Pool.QueryRow(ctx, contentSelect(true)+` WHERE c.id=$1::uuid`, id), true)
}

func (s *Store) ContentDetail(ctx context.Context, id string, userID string) (models.Content, error) {
	c, err := s.GetContent(ctx, id)
	if err != nil {
		return c, err
	}
	if userID != "" {
		items := []models.Content{c}
		_ = s.markContentFavorites(ctx, userID, items)
		c = items[0]
	}
	return c, nil
}

func (s *Store) markContentFavorites(ctx context.Context, userID string, items []models.Content) error {
	if userID == "" || len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, c := range items {
		ids = append(ids, c.ID)
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT target_id FROM favorites
		WHERE user_id=$1::uuid AND target_type='content' AND target_id = ANY($2::uuid[])`, userID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	fav := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		fav[id] = true
	}
	for i := range items {
		items[i].Favorited = fav[items[i].ID]
	}
	return rows.Err()
}

func (s *Store) DecorateContentFavorites(ctx context.Context, userID string, items []models.Content) {
	_ = s.markContentFavorites(ctx, userID, items)
}

func (s *Store) ContentsForWork(ctx context.Context, workID, userID string) ([]models.Content, error) {
	items, _, err := s.ListContents(ctx, ContentFilter{WorkID: workID, Limit: 30})
	if err != nil {
		return nil, err
	}
	_ = s.markContentFavorites(ctx, userID, items)
	return items, nil
}

func (s *Store) ContentsForCharacter(ctx context.Context, characterID, userID string) ([]models.Content, error) {
	items, _, err := s.ListContents(ctx, ContentFilter{CharacterID: characterID, Limit: 30})
	if err != nil {
		return nil, err
	}
	_ = s.markContentFavorites(ctx, userID, items)
	return items, nil
}

type ContentInput struct {
	ArtistID     string    `json:"artist_id"`
	SourceID     string    `json:"source_id"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	ContentType  string    `json:"content_type"`
	SourceAuthor string    `json:"source_author"`
	SourceURL    string    `json:"source_url"`
	CoverURL     string    `json:"cover_url"`
	PublishedAt  time.Time `json:"published_at"`
	IsOfficial   bool      `json:"is_official"`
	TrustLevel   string    `json:"trust_level"`
	SceneIDs     []string  `json:"scene_ids"`
	WorkIDs      []string  `json:"work_ids"`
	CharacterIDs []string  `json:"character_ids"`
	EventIDs     []string  `json:"event_ids"`
}

func (s *Store) CreateContent(ctx context.Context, in ContentInput, userID string) (models.Content, error) {
	if strings.TrimSpace(in.Title) == "" {
		return models.Content{}, fmt.Errorf("title is required")
	}
	if in.ContentType == "" {
		in.ContentType = "internal"
	}
	if in.TrustLevel == "" {
		in.TrustLevel = "internal"
	}
	if in.PublishedAt.IsZero() {
		in.PublishedAt = time.Now()
	}
	urlHash := hashURL(in.SourceURL)
	var id string
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO contents (artist_id, source_id, title, summary, content_type, source_author, source_url,
				cover_url, published_at, discovered_at, published_precision, collected_at, trust_level, is_official,
				review_status, content_status, url_hash, raw_data)
			VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8, $9, now(), 'verified', now(), $10, $11, 'approved', 'published', NULLIF($12,''), '{"manual":true}'::jsonb)
			RETURNING id`,
			in.ArtistID, in.SourceID, in.Title, in.Summary, in.ContentType, in.SourceAuthor, in.SourceURL,
			in.CoverURL, in.PublishedAt, in.TrustLevel, in.IsOfficial, urlHash,
		).Scan(&id); err != nil {
			if strings.Contains(err.Error(), "contents_url_hash_uidx") {
				return ErrConflict
			}
			return err
		}
		if err := replaceRelations(ctx, tx, id, in.SceneIDs, in.WorkIDs, in.CharacterIDs, in.EventIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO review_logs (content_id, user_id, action, before, after, note)
			VALUES ($1::uuid, NULLIF($2,'')::uuid, 'create_manual', NULL, $3, '管理员手动录入')`,
			id, userID, jsonBytes(in))
		return err
	})
	if err != nil {
		return models.Content{}, err
	}
	return s.GetContentAdmin(ctx, id)
}

func replaceRelations(ctx context.Context, tx pgx.Tx, contentID string, sceneIDs, workIDs, charIDs, eventIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM content_scenes WHERE content_id=$1::uuid`, contentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM content_works WHERE content_id=$1::uuid`, contentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM content_characters WHERE content_id=$1::uuid`, contentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM content_events WHERE content_id=$1::uuid`, contentID); err != nil {
		return err
	}
	for _, sid := range sceneIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO content_scenes (content_id, scene_id) VALUES ($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, contentID, sid); err != nil {
			return err
		}
	}
	for _, wid := range workIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO content_works (content_id, work_id) VALUES ($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, contentID, wid); err != nil {
			return err
		}
	}
	for _, cid := range charIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO content_characters (content_id, character_id) VALUES ($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, contentID, cid); err != nil {
			return err
		}
	}
	for _, eid := range eventIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO content_events (content_id, event_id) VALUES ($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, contentID, eid); err != nil {
			return err
		}
	}
	return nil
}

type ContentPatch struct {
	Title              *string    `json:"title"`
	Summary            *string    `json:"summary"`
	ContentType        *string    `json:"content_type"`
	SourceURL          *string    `json:"source_url"`
	CoverURL           *string    `json:"cover_url"`
	IsOfficial         *bool      `json:"is_official"`
	TrustLevel         *string    `json:"trust_level"`
	ContentStatus      *string    `json:"content_status"`
	ReviewStatus       *string    `json:"review_status"`
	ReviewReason       *string    `json:"review_reason"`
	PublishedAt        *time.Time `json:"published_at"`
	ClearPublishedAt   bool       `json:"clear_published_at"`
	PublishedPrecision *string    `json:"published_precision"`
	SceneIDs           *[]string  `json:"scene_ids"`
	WorkIDs            *[]string  `json:"work_ids"`
	CharacterIDs       *[]string  `json:"character_ids"`
	EventIDs           *[]string  `json:"event_ids"`
}

func (s *Store) UpdateContent(ctx context.Context, id, userID string, p ContentPatch, note string) (models.Content, error) {
	var after models.Content
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		before, err := scanContentRow(tx.QueryRow(ctx, contentSelect(true)+` WHERE c.id=$1::uuid`, id), true)
		if err != nil {
			return err
		}
		urlHash := ""
		if p.SourceURL != nil {
			urlHash = hashURL(*p.SourceURL)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE contents SET
				title = COALESCE($2, title),
				summary = COALESCE($3, summary),
				content_type = COALESCE($4, content_type),
				source_url = COALESCE($5, source_url),
				cover_url = COALESCE($6, cover_url),
				is_official = COALESCE($7, is_official),
				trust_level = COALESCE($8, trust_level),
				content_status = COALESCE($9, content_status),
				review_status = COALESCE($10, review_status),
				review_reason = COALESCE($11, review_reason),
				url_hash = CASE WHEN $12::text = '' THEN url_hash ELSE $12 END,
				published_at = CASE WHEN $13::boolean THEN NULL ELSE COALESCE($14::timestamptz, published_at) END,
				published_precision = CASE WHEN $13::boolean THEN 'unknown' ELSE COALESCE($15, published_precision) END,
				updated_at = now()
			WHERE id=$1::uuid`,
			id, p.Title, p.Summary, p.ContentType, p.SourceURL, p.CoverURL, p.IsOfficial, p.TrustLevel,
			p.ContentStatus, p.ReviewStatus, p.ReviewReason, urlHash,
			p.ClearPublishedAt, p.PublishedAt, p.PublishedPrecision,
		); err != nil {
			if strings.Contains(err.Error(), "contents_url_hash_uidx") {
				return ErrConflict
			}
			return err
		}
		if p.SceneIDs != nil || p.WorkIDs != nil || p.CharacterIDs != nil || p.EventIDs != nil {
			var sceneIDs, workIDs, charIDs, eventIDs []string
			if p.SceneIDs != nil {
				sceneIDs = *p.SceneIDs
			} else {
				_ = tx.QueryRow(ctx, `SELECT COALESCE(array_agg(scene_id::text),'{}'::text[]) FROM content_scenes WHERE content_id=$1::uuid`, id).Scan(&sceneIDs)
			}
			if p.WorkIDs != nil {
				workIDs = *p.WorkIDs
			} else {
				_ = tx.QueryRow(ctx, `SELECT COALESCE(array_agg(work_id::text),'{}'::text[]) FROM content_works WHERE content_id=$1::uuid`, id).Scan(&workIDs)
			}
			if p.CharacterIDs != nil {
				charIDs = *p.CharacterIDs
			} else {
				_ = tx.QueryRow(ctx, `SELECT COALESCE(array_agg(character_id::text),'{}'::text[]) FROM content_characters WHERE content_id=$1::uuid`, id).Scan(&charIDs)
			}
			if p.EventIDs != nil {
				eventIDs = *p.EventIDs
			} else {
				_ = tx.QueryRow(ctx, `SELECT COALESCE(array_agg(event_id::text),'{}'::text[]) FROM content_events WHERE content_id=$1::uuid`, id).Scan(&eventIDs)
			}
			if err := replaceRelations(ctx, tx, id, sceneIDs, workIDs, charIDs, eventIDs); err != nil {
				return err
			}
		}
		after, err = scanContentRow(tx.QueryRow(ctx, contentSelect(true)+` WHERE c.id=$1::uuid`, id), true)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO review_logs (content_id, user_id, action, before, after, note)
			VALUES ($1::uuid, NULLIF($2,'')::uuid, 'edit', $3, $4, $5)`,
			id, userID, jsonBytes(before), jsonBytes(after), note)
		return err
	})
	if err != nil {
		return models.Content{}, err
	}
	return after, nil
}

func (s *Store) SetReviewStatus(ctx context.Context, id, userID string, approve bool, reason string) (models.Content, error) {
	var after models.Content
	action := "reject"
	reviewStatus := "rejected"
	contentStatus := "rejected"
	if approve {
		action = "approve"
		reviewStatus = "approved"
		contentStatus = "published"
		reason = ""
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		before, err := scanContentRow(tx.QueryRow(ctx, contentSelect(true)+` WHERE c.id=$1::uuid`, id), true)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE contents SET review_status=$2, content_status=$3, review_reason=$4, updated_at=now()
			WHERE id=$1::uuid`, id, reviewStatus, contentStatus, reason); err != nil {
			return err
		}
		after, err = scanContentRow(tx.QueryRow(ctx, contentSelect(true)+` WHERE c.id=$1::uuid`, id), true)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO review_logs (content_id, user_id, action, before, after, note)
			VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4, $5, $6)`,
			id, userID, action, jsonBytes(before), jsonBytes(after), reason)
		return err
	})
	return after, err
}

func (s *Store) Home(ctx context.Context, userID string) (models.HomeData, error) {
	artist, err := s.GetArtist(ctx)
	if err != nil {
		return models.HomeData{}, err
	}
	scenes, err := s.ListScenes(ctx, userID)
	if err != nil {
		return models.HomeData{}, err
	}
	defaultScene := "all"
	for _, sc := range scenes {
		if sc.IsDefault {
			defaultScene = sc.Code
		}
	}
	latest, _, err := s.ListContents(ctx, ContentFilter{ArtistID: artist.ID, Limit: 6})
	if err != nil {
		return models.HomeData{}, err
	}
	forYou, _, err := s.ListContents(ctx, ContentFilter{ArtistID: artist.ID, Limit: 4})
	if err != nil {
		return models.HomeData{}, err
	}
	if userID != "" {
		f, _, err := s.ListRecommended(ctx, ContentFilter{ArtistID: artist.ID}, userID)
		if err == nil {
			forYou = f
			if len(forYou) > 4 {
				forYou = forYou[:4]
			}
		}
	}
	featured, _, err := s.ListContents(ctx, ContentFilter{ArtistID: artist.ID, ContentType: "internal", Limit: 3})
	if err != nil {
		return models.HomeData{}, err
	}
	works, err := s.ListWorks(ctx, artist.ID, "")
	if err != nil {
		return models.HomeData{}, err
	}
	if len(works) > 6 {
		works = works[:6]
	}
	var brief models.DailyBrief
	brief.Date = time.Now().Format("2006-01-02")
	if err := s.Pool.QueryRow(ctx, `
		SELECT count(*), max(COALESCE(published_at, discovered_at)) FROM contents
		WHERE content_status='published' AND COALESCE(published_at, discovered_at) >= now() - interval '24 hours'`,
	).Scan(&brief.NewCount, &brief.LastUpdated); err != nil {
		return models.HomeData{}, err
	}
	_ = s.markContentFavorites(ctx, userID, latest)
	_ = s.markContentFavorites(ctx, userID, forYou)
	_ = s.markContentFavorites(ctx, userID, featured)
	return models.HomeData{
		Artist: artist, Scenes: scenes, DefaultScene: defaultScene,
		Latest: latest, ForYou: forYou, Featured: featured, Works: works, Brief: brief,
	}, nil
}
