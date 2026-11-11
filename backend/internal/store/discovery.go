package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"xiweispace/backend/internal/models"
)

const providerColumns = `p.id, p.name, p.provider_type, p.endpoint, p.api_key_env, p.config,
	p.enabled, p.last_test_at, p.last_test_ok, p.last_test_error, p.last_latency_ms, p.created_at`

func scanProvider(row rowScanner) (models.SearchProvider, error) {
	var p models.SearchProvider
	var cfg []byte
	err := row.Scan(&p.ID, &p.Name, &p.ProviderType, &p.Endpoint, &p.APIKeyEnv, &cfg,
		&p.Enabled, &p.LastTestAt, &p.LastTestOK, &p.LastTestError, &p.LastLatencyMS, &p.CreatedAt)
	if err != nil {
		return p, mapNoRows(err)
	}
	p.Config = map[string]any{}
	_ = json.Unmarshal(cfg, &p.Config)
	return p, nil
}

func (s *Store) ListSearchProviders(ctx context.Context) ([]models.SearchProvider, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+providerColumns+` FROM search_providers p ORDER BY p.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.SearchProvider, 0)
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetSearchProvider(ctx context.Context, id string) (models.SearchProvider, error) {
	return scanProvider(s.Pool.QueryRow(ctx, `SELECT `+providerColumns+` FROM search_providers p WHERE p.id=$1::uuid`, id))
}

type SearchProviderInput struct {
	Name         string         `json:"name"`
	ProviderType string         `json:"provider_type"`
	Endpoint     string         `json:"endpoint"`
	APIKeyEnv    string         `json:"api_key_env"`
	Config       map[string]any `json:"config"`
	Enabled      *bool          `json:"enabled"`
}

func (s *Store) CreateSearchProvider(ctx context.Context, in SearchProviderInput) (models.SearchProvider, error) {
	if strings.TrimSpace(in.Name) == "" {
		return models.SearchProvider{}, fmt.Errorf("名称不能为空")
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	cfg, _ := json.Marshal(in.Config)
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO search_providers (name, provider_type, endpoint, api_key_env, config, enabled)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6) RETURNING id`,
		in.Name, in.ProviderType, in.Endpoint, in.APIKeyEnv, cfg, enabled).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "search_providers_name_key") {
			return models.SearchProvider{}, ErrConflict
		}
		return models.SearchProvider{}, err
	}
	return s.GetSearchProvider(ctx, id)
}

type SearchProviderPatch struct {
	Name         *string        `json:"name"`
	ProviderType *string        `json:"provider_type"`
	Endpoint     *string        `json:"endpoint"`
	APIKeyEnv    *string        `json:"api_key_env"`
	Config       map[string]any `json:"config"`
	Enabled      *bool          `json:"enabled"`
}

func (s *Store) UpdateSearchProvider(ctx context.Context, id string, p SearchProviderPatch) (models.SearchProvider, error) {
	var cfg []byte
	if p.Config != nil {
		cfg, _ = json.Marshal(p.Config)
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE search_providers SET
			name=COALESCE($2,name), provider_type=COALESCE($3,provider_type),
			endpoint=COALESCE($4,endpoint), api_key_env=COALESCE($5,api_key_env),
			config=COALESCE($6::jsonb,config), enabled=COALESCE($7,enabled), updated_at=now()
		WHERE id=$1::uuid`,
		id, p.Name, p.ProviderType, p.Endpoint, p.APIKeyEnv, nullJSON(cfg), p.Enabled)
	if err != nil {
		if strings.Contains(err.Error(), "search_providers_name_key") {
			return models.SearchProvider{}, ErrConflict
		}
		return models.SearchProvider{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.SearchProvider{}, ErrNotFound
	}
	return s.GetSearchProvider(ctx, id)
}

func (s *Store) RecordProviderTest(ctx context.Context, id string, ok bool, latencyMS int, message string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE search_providers SET last_test_at=now(), last_test_ok=$2, last_test_error=$3,
			last_latency_ms=$4, updated_at=now() WHERE id=$1::uuid`, id, ok, message, latencyMS)
	return err
}

const taskSelect = `
	SELECT t.id, t.name, t.provider_id, p.name, t.query, t.domains, t.content_type_hint,
	       t.source_type, t.trust_level, t.interval_minutes, t.max_results, t.enabled,
	       t.last_run_at, t.last_success_at, t.last_failure_at, t.last_error, t.next_run_at,
	       t.created_at
	FROM search_tasks t JOIN search_providers p ON p.id=t.provider_id`

func scanTask(row rowScanner) (models.SearchTask, error) {
	var t models.SearchTask
	err := row.Scan(&t.ID, &t.Name, &t.ProviderID, &t.ProviderName, &t.Query, &t.Domains,
		&t.ContentTypeHint, &t.SourceType, &t.TrustLevel, &t.IntervalMinutes, &t.MaxResults,
		&t.Enabled, &t.LastRunAt, &t.LastSuccessAt, &t.LastFailureAt, &t.LastError,
		&t.NextRunAt, &t.CreatedAt)
	return t, mapNoRows(err)
}

func (s *Store) ListSearchTasks(ctx context.Context) ([]models.SearchTask, error) {
	rows, err := s.Pool.Query(ctx, taskSelect+` ORDER BY t.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.SearchTask, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetSearchTask(ctx context.Context, id string) (models.SearchTask, error) {
	return scanTask(s.Pool.QueryRow(ctx, taskSelect+` WHERE t.id=$1::uuid`, id))
}

type SearchTaskInput struct {
	Name            string   `json:"name"`
	ProviderID      string   `json:"provider_id"`
	Query           string   `json:"query"`
	Domains         []string `json:"domains"`
	ContentTypeHint string   `json:"content_type_hint"`
	SourceType      string   `json:"source_type"`
	TrustLevel      string   `json:"trust_level"`
	IntervalMinutes int      `json:"interval_minutes"`
	MaxResults      int      `json:"max_results"`
	Enabled         *bool    `json:"enabled"`
}

func normalizeTaskInput(in SearchTaskInput) SearchTaskInput {
	if len(in.Domains) == 0 {
		in.Domains = []string{}
	}
	if in.SourceType == "" {
		in.SourceType = "media"
	}
	if in.TrustLevel == "" {
		in.TrustLevel = "media"
	}
	if in.IntervalMinutes < 15 {
		in.IntervalMinutes = 360
	}
	if in.MaxResults < 1 {
		in.MaxResults = 20
	}
	if in.MaxResults > 50 {
		in.MaxResults = 50
	}
	return in
}

func (s *Store) CreateSearchTask(ctx context.Context, in SearchTaskInput) (models.SearchTask, error) {
	in = normalizeTaskInput(in)
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Query) == "" || in.ProviderID == "" {
		return models.SearchTask{}, fmt.Errorf("名称、搜索服务与关键词不能为空")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO search_tasks (name, provider_id, query, domains, content_type_hint,
			source_type, trust_level, interval_minutes, max_results, enabled, next_run_at)
		VALUES ($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		in.Name, in.ProviderID, in.Query, in.Domains, in.ContentTypeHint, in.SourceType,
		in.TrustLevel, in.IntervalMinutes, in.MaxResults, enabled, time.Now()).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "search_tasks_name_key") {
			return models.SearchTask{}, ErrConflict
		}
		return models.SearchTask{}, err
	}
	return s.GetSearchTask(ctx, id)
}

type SearchTaskPatch struct {
	Name            *string   `json:"name"`
	ProviderID      *string   `json:"provider_id"`
	Query           *string   `json:"query"`
	Domains         *[]string `json:"domains"`
	ContentTypeHint *string   `json:"content_type_hint"`
	SourceType      *string   `json:"source_type"`
	TrustLevel      *string   `json:"trust_level"`
	IntervalMinutes *int      `json:"interval_minutes"`
	MaxResults      *int      `json:"max_results"`
	Enabled         *bool     `json:"enabled"`
}

func (s *Store) UpdateSearchTask(ctx context.Context, id string, p SearchTaskPatch) (models.SearchTask, error) {
	old, err := s.GetSearchTask(ctx, id)
	if err != nil {
		return models.SearchTask{}, err
	}
	nextRun := old.NextRunAt
	if p.Enabled != nil && *p.Enabled && !old.Enabled {
		nextRun = time.Now()
	}
	if p.IntervalMinutes != nil && *p.IntervalMinutes != old.IntervalMinutes {
		nextRun = time.Now()
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE search_tasks SET
			name=COALESCE($2,name), provider_id=COALESCE($3::uuid,provider_id),
			query=COALESCE($4,query), domains=COALESCE($5,domains),
			content_type_hint=COALESCE($6,content_type_hint), source_type=COALESCE($7,source_type),
			trust_level=COALESCE($8,trust_level), interval_minutes=COALESCE($9,interval_minutes),
			max_results=COALESCE($10,max_results), enabled=COALESCE($11,enabled),
			next_run_at=$12, updated_at=now()
		WHERE id=$1::uuid`,
		id, p.Name, p.ProviderID, p.Query, p.Domains, p.ContentTypeHint, p.SourceType,
		p.TrustLevel, p.IntervalMinutes, p.MaxResults, p.Enabled, nextRun)
	if err != nil {
		if strings.Contains(err.Error(), "search_tasks_name_key") {
			return models.SearchTask{}, ErrConflict
		}
		return models.SearchTask{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.SearchTask{}, ErrNotFound
	}
	return s.GetSearchTask(ctx, id)
}

func (s *Store) DeleteSearchTask(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM search_tasks WHERE id=$1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) QueueSearchRun(ctx context.Context, taskID, trigger string) (string, error) {
	if trigger != "manual" && trigger != "retry" && trigger != "schedule" {
		trigger = "manual"
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM search_tasks WHERE id=$1::uuid)`, taskID).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", ErrNotFound
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO search_runs (task_id, status, trigger_type)
		VALUES ($1::uuid, 'queued', $2)
		ON CONFLICT (task_id) WHERE status IN ('queued','running') DO NOTHING
		RETURNING id`, taskID, trigger).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = s.Pool.QueryRow(ctx, `
		SELECT id FROM search_runs WHERE task_id=$1::uuid AND status IN ('queued','running')
		ORDER BY created_at DESC LIMIT 1`, taskID).Scan(&id)
	return id, err
}

func (s *Store) CreateDueSearchRuns(ctx context.Context) (int, error) {
	tag, err := s.Pool.Exec(ctx, `
		WITH due AS (
			SELECT t.id FROM search_tasks t
			WHERE t.enabled = true AND t.next_run_at <= now()
			  AND NOT EXISTS (
				SELECT 1 FROM search_runs r
				WHERE r.task_id=t.id AND r.status IN ('queued','running')
			  )
			FOR UPDATE SKIP LOCKED
		), inserted AS (
			INSERT INTO search_runs (task_id, status, trigger_type)
			SELECT id, 'queued', 'schedule' FROM due
			ON CONFLICT DO NOTHING
			RETURNING task_id
		)
		UPDATE search_tasks t
		SET next_run_at = now() + (t.interval_minutes || ' minutes')::interval
		FROM inserted i WHERE t.id = i.task_id`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

const runSelect = `
	SELECT r.id, r.task_id, t.name, r.status, r.trigger_type, r.started_at, r.finished_at,
	       r.fetched_count, r.inserted_count, r.duplicate_count, r.failed_count,
	       r.parsed_ok_count, r.parse_failed_count, r.error_message, r.created_at
	FROM search_runs r JOIN search_tasks t ON t.id=r.task_id`

func scanRun(row rowScanner) (models.SearchRun, error) {
	var r models.SearchRun
	err := row.Scan(&r.ID, &r.TaskID, &r.TaskName, &r.Status, &r.TriggerType, &r.StartedAt,
		&r.FinishedAt, &r.FetchedCount, &r.InsertedCount, &r.DuplicateCount, &r.FailedCount,
		&r.ParsedOKCount, &r.ParseFailedCount, &r.ErrorMessage, &r.CreatedAt)
	return r, mapNoRows(err)
}

func (s *Store) ListSearchRuns(ctx context.Context, limit int) ([]models.SearchRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, runSelect+` ORDER BY r.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.SearchRun, 0)
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetSearchRun(ctx context.Context, id string) (models.SearchRun, error) {
	return scanRun(s.Pool.QueryRow(ctx, runSelect+` WHERE r.id=$1::uuid`, id))
}

func (s *Store) ClaimSearchRun(ctx context.Context) (models.SearchRun, models.SearchTask, error) {
	var runID, taskID string
	err := s.Pool.QueryRow(ctx, `
		WITH next AS (
			SELECT id FROM search_runs WHERE status='queued'
			ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		UPDATE search_runs r SET status='running', started_at=now(), heartbeat_at=now()
		FROM next WHERE r.id=next.id
		RETURNING r.id, r.task_id`).Scan(&runID, &taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.SearchRun{}, models.SearchTask{}, ErrNotFound
	}
	if err != nil {
		return models.SearchRun{}, models.SearchTask{}, fmt.Errorf("claim search run: %w", err)
	}
	run, err := s.GetSearchRun(ctx, runID)
	if err != nil {
		s.failClaimedSearchRun(runID, "领取后读取运行记录失败: "+err.Error())
		return models.SearchRun{}, models.SearchTask{}, fmt.Errorf("load claimed search run %s: %w", runID, err)
	}
	task, err := s.GetSearchTask(ctx, taskID)
	if err != nil {
		s.failClaimedSearchRun(runID, "领取后读取搜索任务失败: "+err.Error())
		return models.SearchRun{}, models.SearchTask{}, fmt.Errorf("load search task %s for claimed run %s: %w", taskID, runID, err)
	}
	return run, task, nil
}

// failClaimedSearchRun closes a run whose related task could not be loaded, so
// it cannot remain running until stale recovery.
func (s *Store) failClaimedSearchRun(runID, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.Pool.Exec(ctx, `
		UPDATE search_runs SET status='failed', finished_at=now(), error_message=$2
		WHERE id=$1::uuid AND status='running'`, runID, message)
}

// CompleteSearchRun finalizes a search run. A run is only successful when no
// page and no candidate failed. Partial and failed runs update last_failure_at
// but deliberately preserve last_success_at, so the overlap window keeps
// covering everything since the last complete success. paginationPartial marks
// a later page failure even when no individual candidate failed.
func (s *Store) CompleteSearchRun(ctx context.Context, runID, taskID string, fetched, inserted, duplicates, failed, parsedOK, parseFailed int, errMsg string, runFailed, paginationPartial bool) error {
	status := "success"
	switch {
	case runFailed:
		status = "failed"
	case paginationPartial || failed > 0:
		status = "partial"
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE search_runs SET status=$2, finished_at=now(), fetched_count=$3,
				inserted_count=$4, duplicate_count=$5, failed_count=$6,
				parsed_ok_count=$7, parse_failed_count=$8, error_message=$9
			WHERE id=$1::uuid`,
			runID, status, fetched, inserted, duplicates, failed, parsedOK, parseFailed, errMsg); err != nil {
			return err
		}
		if status != "success" {
			_, err := tx.Exec(ctx, `
				UPDATE search_tasks SET last_run_at=now(), last_failure_at=now(), last_error=$2, updated_at=now()
				WHERE id=$1::uuid`, taskID, errMsg)
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE search_tasks SET last_run_at=now(), last_success_at=now(), last_error='', updated_at=now()
			WHERE id=$1::uuid`, taskID)
		return err
	})
}

// staleSearchThreshold is higher than the collection threshold because a
// search run may scan up to five result pages and fetch metadata for each
// candidate. It still sits above the 3-minute execution context, so a live
// run is never reclaimed. Heartbeats are refreshed every 30 seconds.
const staleSearchThreshold = 10 * time.Minute

func (s *Store) RecoverStaleSearchRuns(ctx context.Context) (int, error) {
	tag, err := s.Pool.Exec(ctx, `
		WITH stale AS (
			UPDATE search_runs SET status='failed', finished_at=now(),
				error_message='搜索任务心跳超时（超过 10 分钟未更新），已由调度器回收'
			WHERE status='running' AND COALESCE(heartbeat_at, started_at) < now() - ($1 * interval '1 minute')
			RETURNING task_id
		)
		UPDATE search_tasks t SET last_run_at=now(), last_failure_at=now(),
			last_error='搜索任务心跳超时（超过 10 分钟未更新），已由调度器回收', updated_at=now()
		FROM stale WHERE t.id=stale.task_id`,
		int(staleSearchThreshold.Minutes()))
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// HeartbeatSearchRun marks a claimed search run as still alive.
func (s *Store) HeartbeatSearchRun(ctx context.Context, runID string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE search_runs SET heartbeat_at=now()
		WHERE id=$1::uuid AND status='running'`, runID)
	return err
}

type DiscoveredContent struct {
	SearchTaskID       string
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
	OriginName         string
	OriginPlatform     string
	OriginType         string
	MetadataStatus     string
	MetadataError      string
	TrustLevel         string
	IsOfficial         bool
	ReviewStatus       string
	ContentStatus      string
	ReviewReason       string
	RelevanceScore     int
	RelevanceLabel     string
	FilterReason       string
	RawData            []byte
	SceneIDs           []string
}

// InsertDiscovered inserts a search candidate. Unlike RSS collection it keeps
// published_at NULL when the publication time is unknown and records the
// separate discovery time. Dedupe is by canonical URL plus a 14-day
// title/summary fingerprint, so independent reports about the same event
// remain separate timeline entries while syndicated copies do not.
func (s *Store) InsertDiscovered(ctx context.Context, c DiscoveredContent) (string, error) {
	status := ""
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var existing string
		if c.URLHash != "" {
			err := tx.QueryRow(ctx, `SELECT id FROM contents WHERE url_hash=$1 LIMIT 1`, c.URLHash).Scan(&existing)
			if err == nil {
				status = Duplicate
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		// Cross-provider dedupe: the same article can be returned by several
		// search providers with different URLs. A 14-day fingerprint window
		// keeps syndicated copies out without merging independent reports.
		if c.ContentHash != "" {
			err := tx.QueryRow(ctx, `
				SELECT id FROM contents
				WHERE content_hash=$1 AND discovered_at > now() - interval '14 days'
				LIMIT 1`, c.ContentHash).Scan(&existing)
			if err == nil {
				status = Duplicate
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if c.PublishedPrecision == "" {
			c.PublishedPrecision = "unknown"
		}
		if c.RawData == nil {
			c.RawData = []byte("{}")
		}
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO contents (
				artist_id, source_id, search_task_id, title, summary, content_type, source_author,
				source_url, cover_url, published_at, discovered_at, published_precision, collected_at,
				origin_name, origin_platform, origin_type, metadata_status, metadata_error,
				trust_level, is_official, review_status, content_status, review_reason,
				url_hash, content_hash, relevance_score, relevance_label, filter_reason, raw_data
			)
			SELECT a.id, NULL, $1::uuid, $2, $3, $4, $5, $6, $7, $8, now(), $9, now(),
			       $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			       NULLIF($20,''), NULLIF($21,''), $22, $23, $24, $25::jsonb
			FROM artists a ORDER BY a.created_at LIMIT 1
			RETURNING id`,
			c.SearchTaskID, c.Title, c.Summary, c.ContentType, c.Author, c.URL, c.CoverURL,
			c.PublishedAt, c.PublishedPrecision, c.OriginName, c.OriginPlatform, c.OriginType,
			c.MetadataStatus, c.MetadataError, c.TrustLevel, c.IsOfficial, c.ReviewStatus,
			c.ContentStatus, c.ReviewReason, c.URLHash, c.ContentHash,
			c.RelevanceScore, c.RelevanceLabel, c.FilterReason, c.RawData,
		).Scan(&id); err != nil {
			if strings.Contains(err.Error(), "contents_url_hash_uidx") {
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

type Facet struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

func (s *Store) ContentFacets(ctx context.Context) (map[string][]Facet, error) {
	out := map[string][]Facet{"platforms": {}, "content_types": {}, "source_types": {}}
	queries := []struct {
		key  string
		sql  string
		args []any
	}{
		{
			key: "platforms",
			sql: `SELECT COALESCE(NULLIF(c.origin_platform,''), s.platform, ''), count(*)
				FROM contents c LEFT JOIN sources s ON s.id=c.source_id
				WHERE c.content_status='published'
				GROUP BY 1 HAVING COALESCE(NULLIF(c.origin_platform,''), s.platform, '') <> ''
				ORDER BY count(*) DESC LIMIT 30`,
		},
		{
			key: "content_types",
			sql: `SELECT c.content_type, count(*) FROM contents c
				WHERE c.content_status='published'
				GROUP BY 1 ORDER BY count(*) DESC`,
		},
		{
			key: "source_types",
			sql: `SELECT COALESCE(NULLIF(c.origin_type,''), s.source_type, 'internal'), count(*)
				FROM contents c LEFT JOIN sources s ON s.id=c.source_id
				WHERE c.content_status='published'
				GROUP BY 1 HAVING COALESCE(NULLIF(c.origin_type,''), s.source_type, 'internal') <> ''
				ORDER BY count(*) DESC`,
		},
	}
	for _, q := range queries {
		rows, err := s.Pool.Query(ctx, q.sql, q.args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var f Facet
			if err := rows.Scan(&f.Value, &f.Count); err != nil {
				rows.Close()
				return nil, err
			}
			f.Label = f.Value
			out[q.key] = append(out[q.key], f)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
