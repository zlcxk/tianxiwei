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

const sourceColumns = `s.id, s.name, s.platform, s.source_type, s.adapter_type, s.config,
	s.trust_level, s.enabled, s.interval_minutes, s.last_success_at, s.last_failure_at,
	s.last_error, s.created_at`

func scanSource(row rowScanner) (models.Source, error) {
	var src models.Source
	var cfg []byte
	err := row.Scan(&src.ID, &src.Name, &src.Platform, &src.SourceType, &src.AdapterType, &cfg,
		&src.TrustLevel, &src.Enabled, &src.IntervalMinutes, &src.LastSuccessAt, &src.LastFailureAt,
		&src.LastError, &src.CreatedAt)
	if err != nil {
		return src, mapNoRows(err)
	}
	src.Config = map[string]any{}
	_ = json.Unmarshal(cfg, &src.Config)
	return src, nil
}

func (s *Store) ListSources(ctx context.Context) ([]models.Source, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+sourceColumns+` FROM sources s ORDER BY s.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Source, 0)
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

func (s *Store) GetSource(ctx context.Context, id string) (models.Source, error) {
	return scanSource(s.Pool.QueryRow(ctx, `SELECT `+sourceColumns+` FROM sources s WHERE s.id=$1::uuid`, id))
}

type SourceInput struct {
	Name            string         `json:"name"`
	Platform        string         `json:"platform"`
	SourceType      string         `json:"source_type"`
	AdapterType     string         `json:"adapter_type"`
	Config          map[string]any `json:"config"`
	TrustLevel      string         `json:"trust_level"`
	Enabled         *bool          `json:"enabled"`
	IntervalMinutes int            `json:"interval_minutes"`
}

func normalizeSourceInput(in SourceInput) SourceInput {
	if in.SourceType == "" {
		in.SourceType = "media"
	}
	if in.AdapterType == "" {
		in.AdapterType = "manual"
	}
	if in.TrustLevel == "" {
		in.TrustLevel = in.SourceType
	}
	if in.IntervalMinutes < 5 {
		in.IntervalMinutes = 120
	}
	if in.Config == nil {
		in.Config = map[string]any{}
	}
	return in
}

func (s *Store) CreateSource(ctx context.Context, in SourceInput) (models.Source, error) {
	in = normalizeSourceInput(in)
	if strings.TrimSpace(in.Name) == "" {
		return models.Source{}, fmt.Errorf("name is required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	cfg, _ := json.Marshal(in.Config)
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO sources (name, platform, source_type, adapter_type, config, trust_level, enabled, interval_minutes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		in.Name, in.Platform, in.SourceType, in.AdapterType, cfg, in.TrustLevel, enabled, in.IntervalMinutes,
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "sources_name_key") {
			return models.Source{}, ErrConflict
		}
		return models.Source{}, err
	}
	return s.GetSource(ctx, id)
}

type SourcePatch struct {
	Name            *string        `json:"name"`
	Platform        *string        `json:"platform"`
	SourceType      *string        `json:"source_type"`
	AdapterType     *string        `json:"adapter_type"`
	Config          map[string]any `json:"config"`
	TrustLevel      *string        `json:"trust_level"`
	Enabled         *bool          `json:"enabled"`
	IntervalMinutes *int           `json:"interval_minutes"`
}

func (s *Store) UpdateSource(ctx context.Context, id string, p SourcePatch) (models.Source, error) {
	var cfg []byte
	if p.Config != nil {
		cfg, _ = json.Marshal(p.Config)
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE sources SET
			name=COALESCE($2,name), platform=COALESCE($3,platform),
			source_type=COALESCE($4,source_type), adapter_type=COALESCE($5,adapter_type),
			config=COALESCE($6::jsonb,config), trust_level=COALESCE($7,trust_level),
			enabled=COALESCE($8,enabled), interval_minutes=COALESCE($9,interval_minutes),
			updated_at=now()
		WHERE id=$1::uuid`,
		id, p.Name, p.Platform, p.SourceType, p.AdapterType, nullJSON(cfg), p.TrustLevel, p.Enabled, p.IntervalMinutes)
	if err != nil {
		result := mapNoRows(err)
		if result == ErrNotFound {
			return models.Source{}, result
		}
		return models.Source{}, err
	}
	return s.GetSource(ctx, id)
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func (s *Store) QueueJob(ctx context.Context, sourceID, trigger string) (string, error) {
	if trigger != "manual" && trigger != "retry" && trigger != "schedule" {
		trigger = "manual"
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sources WHERE id=$1::uuid)`, sourceID).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", ErrNotFound
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO collection_jobs (source_id, status, trigger_type)
		VALUES ($1::uuid, 'queued', $2)
		ON CONFLICT (source_id) WHERE status IN ('queued','running') DO NOTHING
		RETURNING id`, sourceID, trigger).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = s.Pool.QueryRow(ctx, `
		SELECT id FROM collection_jobs WHERE source_id=$1::uuid AND status IN ('queued','running')
		ORDER BY created_at DESC LIMIT 1`, sourceID).Scan(&id)
	return id, err
}

func (s *Store) CreateDueJobs(ctx context.Context) (int, error) {
	tag, err := s.Pool.Exec(ctx, `
		INSERT INTO collection_jobs (source_id, status, trigger_type)
		SELECT s.id, 'queued', 'schedule'
		FROM sources s
		WHERE s.enabled = true AND s.adapter_type <> 'manual'
		  AND GREATEST(
			COALESCE(s.last_success_at, 'epoch'::timestamptz),
			COALESCE(s.last_failure_at, 'epoch'::timestamptz)
		  ) < now() - (s.interval_minutes || ' minutes')::interval
		  AND NOT EXISTS (
			SELECT 1 FROM collection_jobs j
			WHERE j.source_id=s.id AND j.status IN ('queued','running')
		  )`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// staleJobThreshold is well above the 3-minute execution context in Processor,
// so a live task is never reclaimed while it can still be running. Heartbeats
// are refreshed every 30 seconds; a crashed worker's jobs become reclaimable
// after this window.
const staleJobThreshold = 8 * time.Minute

func (s *Store) RecoverStaleJobs(ctx context.Context) (int, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE collection_jobs SET status='failed', finished_at=now(),
			error_message='任务心跳超时（超过 8 分钟未更新），已由调度器回收'
		WHERE status='running' AND COALESCE(heartbeat_at, started_at) < now() - ($1 * interval '1 minute')`,
		int(staleJobThreshold.Minutes()))
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// HeartbeatJob marks a claimed collection job as still alive. Errors are
// returned so the caller can log them; a failed heartbeat leaves the previous
// timestamp in place rather than corrupting the task state.
func (s *Store) HeartbeatJob(ctx context.Context, jobID string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE collection_jobs SET heartbeat_at=now()
		WHERE id=$1::uuid AND status='running'`, jobID)
	return err
}

func (s *Store) ClaimJob(ctx context.Context) (models.Job, models.Source, error) {
	var jobID, sourceID string
	err := s.Pool.QueryRow(ctx, `
		WITH next AS (
			SELECT id FROM collection_jobs WHERE status='queued'
			ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		UPDATE collection_jobs j SET status='running', started_at=now(), heartbeat_at=now()
		FROM next WHERE j.id=next.id
		RETURNING j.id, j.source_id`).Scan(&jobID, &sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Job{}, models.Source{}, ErrNotFound
	}
	if err != nil {
		return models.Job{}, models.Source{}, fmt.Errorf("claim collection job: %w", err)
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		s.failClaimedJob(jobID, "领取后读取任务失败: "+err.Error())
		return models.Job{}, models.Source{}, fmt.Errorf("load claimed collection job %s: %w", jobID, err)
	}
	src, err := s.GetSource(ctx, sourceID)
	if err != nil {
		s.failClaimedJob(jobID, "领取后读取来源失败: "+err.Error())
		return models.Job{}, models.Source{}, fmt.Errorf("load source %s for claimed job %s: %w", sourceID, jobID, err)
	}
	return job, src, nil
}

// failClaimedJob closes a job that was moved to running but whose related data
// could not be loaded. This prevents a broken relation from leaving the task
// stuck in running until stale recovery.
func (s *Store) failClaimedJob(jobID, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.Pool.Exec(ctx, `
		UPDATE collection_jobs SET status='failed', finished_at=now(), error_message=$2
		WHERE id=$1::uuid AND status='running'`, jobID, message)
}

func (s *Store) CompleteJob(ctx context.Context, jobID, sourceID string, fetched, inserted, duplicates, failed int, errMsg string, jobFailed bool) error {
	status := "success"
	switch {
	case jobFailed:
		status = "failed"
	case failed > 0:
		status = "partial"
	}
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE collection_jobs SET status=$2, finished_at=now(), fetched_count=$3,
				inserted_count=$4, duplicate_count=$5, failed_count=$6, error_message=$7
			WHERE id=$1::uuid`,
			jobID, status, fetched, inserted, duplicates, failed, errMsg); err != nil {
			return err
		}
		if jobFailed {
			_, err := tx.Exec(ctx, `UPDATE sources SET last_failure_at=now(), last_error=$2, updated_at=now() WHERE id=$1::uuid`, sourceID, errMsg)
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE sources SET last_success_at=now(), last_error='', updated_at=now() WHERE id=$1::uuid`, sourceID)
		return err
	})
}

const jobSelect = `
	SELECT j.id, j.source_id, s.name, j.status, j.trigger_type, j.started_at, j.finished_at,
	       j.fetched_count, j.inserted_count, j.duplicate_count, j.failed_count,
	       j.error_message, j.created_at
	FROM collection_jobs j JOIN sources s ON s.id=j.source_id`

func scanJob(row rowScanner) (models.Job, error) {
	var j models.Job
	err := row.Scan(&j.ID, &j.SourceID, &j.SourceName, &j.Status, &j.TriggerType, &j.StartedAt,
		&j.FinishedAt, &j.FetchedCount, &j.InsertedCount, &j.DuplicateCount, &j.FailedCount,
		&j.ErrorMessage, &j.CreatedAt)
	return j, mapNoRows(err)
}

func (s *Store) GetJob(ctx context.Context, id string) (models.Job, error) {
	return scanJob(s.Pool.QueryRow(ctx, jobSelect+` WHERE j.id=$1::uuid`, id))
}

func (s *Store) ListJobs(ctx context.Context, limit int, status string) ([]models.Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, jobSelect+`
		WHERE ($1='' OR j.status=$1)
		ORDER BY j.created_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Job, 0)
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) RetryJob(ctx context.Context, jobID string) (string, error) {
	var sourceID string
	err := s.Pool.QueryRow(ctx, `SELECT source_id FROM collection_jobs WHERE id=$1::uuid`, jobID).Scan(&sourceID)
	if err != nil {
		return "", mapNoRows(err)
	}
	return s.QueueJob(ctx, sourceID, "retry")
}

type Rule struct {
	ID        string `json:"id"`
	SceneID   string `json:"scene_id"`
	SceneName string `json:"scene_name"`
	Keywords  string `json:"keywords"`
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
}

func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.id, r.scene_id, sc.name, r.keywords, r.priority, r.enabled
		FROM classification_rules r JOIN scenes sc ON sc.id=r.scene_id
		ORDER BY r.priority DESC, sc.sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Rule, 0)
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.SceneID, &r.SceneName, &r.Keywords, &r.Priority, &r.Enabled); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type RuleInput struct {
	SceneID  string `json:"scene_id"`
	Keywords string `json:"keywords"`
	Priority int    `json:"priority"`
	Enabled  *bool  `json:"enabled"`
}

func (s *Store) CreateRule(ctx context.Context, in RuleInput) (string, error) {
	if in.SceneID == "" || strings.TrimSpace(in.Keywords) == "" {
		return "", fmt.Errorf("scene_id and keywords are required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	var id string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO classification_rules (scene_id, keywords, priority, enabled)
		VALUES ($1::uuid,$2,$3,$4) RETURNING id`,
		in.SceneID, in.Keywords, in.Priority, enabled).Scan(&id)
	if err != nil && strings.Contains(err.Error(), "classification_rules_scene_id_keywords_key") {
		return "", ErrConflict
	}
	return id, err
}

func (s *Store) UpdateRule(ctx context.Context, id string, in RuleInput) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE classification_rules SET scene_id=$2::uuid, keywords=$3, priority=$4, enabled=COALESCE($5,enabled)
		WHERE id=$1::uuid`, id, in.SceneID, in.Keywords, in.Priority, in.Enabled)
	return err
}

func (s *Store) DeleteRule(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM classification_rules WHERE id=$1::uuid`, id)
	return err
}

type Overview struct {
	TodayCollected int          `json:"today_collected"`
	TodayInserted  int          `json:"today_inserted"`
	PendingReviews int          `json:"pending_reviews"`
	SourceErrors   int          `json:"source_errors"`
	SuccessRate    float64      `json:"success_rate"`
	RecentJobs     []models.Job `json:"recent_jobs"`
}

func (s *Store) GetOverview(ctx context.Context) (Overview, error) {
	var o Overview
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sum(fetched_count),0), COALESCE(sum(inserted_count),0)
		FROM collection_jobs WHERE created_at >= current_date`).Scan(&o.TodayCollected, &o.TodayInserted); err != nil {
		return o, err
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM contents WHERE review_status='pending'`).Scan(&o.PendingReviews); err != nil {
		return o, err
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM sources WHERE enabled=true AND last_error <> ''`).Scan(&o.SourceErrors); err != nil {
		return o, err
	}
	var success, failed, partial int
	if err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status='success'),
		       count(*) FILTER (WHERE status='failed'),
		       count(*) FILTER (WHERE status='partial')
		FROM collection_jobs WHERE created_at >= now() - interval '7 days'`).Scan(&success, &failed, &partial); err != nil {
		return o, err
	}
	if success+failed+partial > 0 {
		o.SuccessRate = float64(success) / float64(success+failed+partial)
	}
	jobs, err := s.ListJobs(ctx, 8, "")
	if err != nil {
		return o, err
	}
	o.RecentJobs = jobs
	return o, nil
}

func (s *Store) GetSetting(ctx context.Context, key string) (json.RawMessage, error) {
	var raw []byte
	err := s.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return json.RawMessage("{}"), nil
	}
	return raw, err
}

func (s *Store) PutSetting(ctx context.Context, key string, value []byte) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ($1,$2)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()`, key, value)
	return err
}

type RecommendWeights struct {
	Scene      int `json:"scene"`
	Recency    int `json:"recency"`
	Trust      int `json:"trust"`
	Popularity int `json:"popularity"`
}

func (s *Store) GetRecommendWeights(ctx context.Context) RecommendWeights {
	w := RecommendWeights{Scene: 40, Recency: 30, Trust: 20, Popularity: 10}
	raw, err := s.GetSetting(ctx, "recommend_weights")
	if err != nil {
		return w
	}
	_ = json.Unmarshal(raw, &w)
	if w.Scene+w.Recency+w.Trust+w.Popularity != 100 || w.Scene < 0 || w.Recency < 0 || w.Trust < 0 || w.Popularity < 0 {
		return RecommendWeights{Scene: 40, Recency: 30, Trust: 20, Popularity: 10}
	}
	return w
}

func (s *Store) PutRecommendWeights(ctx context.Context, w RecommendWeights) error {
	if w.Scene < 0 || w.Recency < 0 || w.Trust < 0 || w.Popularity < 0 || w.Scene+w.Recency+w.Trust+w.Popularity != 100 {
		return fmt.Errorf("推荐权重之和必须为 100，且不能为负数")
	}
	raw, _ := json.Marshal(w)
	return s.PutSetting(ctx, "recommend_weights", raw)
}
