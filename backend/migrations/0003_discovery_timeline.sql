-- 内容发现与时间线：发布时间精度、发现时间、来源元数据、搜索任务、事件关联。
-- 迁移保持向后兼容：已有内容保留原发布时间，并标记为 verified。

-- ---------------------------------------------------------------------------
-- 时间线字段
-- ---------------------------------------------------------------------------
ALTER TABLE contents ALTER COLUMN published_at DROP DEFAULT;
ALTER TABLE contents ALTER COLUMN published_at DROP NOT NULL;

ALTER TABLE contents ADD COLUMN discovered_at TIMESTAMPTZ;
UPDATE contents SET discovered_at = collected_at WHERE discovered_at IS NULL;
ALTER TABLE contents ALTER COLUMN discovered_at SET DEFAULT now();
ALTER TABLE contents ALTER COLUMN discovered_at SET NOT NULL;

ALTER TABLE contents ADD COLUMN published_precision TEXT NOT NULL DEFAULT 'verified'
    CHECK (published_precision IN ('verified', 'estimated', 'unknown'));
UPDATE contents SET published_precision = 'unknown' WHERE published_at IS NULL;

-- 保留历史内容类型，同时扩展与饭圈场景相互独立的类型。
ALTER TABLE contents DROP CONSTRAINT IF EXISTS contents_content_type_check;
ALTER TABLE contents ADD CONSTRAINT contents_content_type_check CHECK (content_type IN (
    'social', 'post', 'news', 'drama', 'variety', 'media', 'video', 'interview',
    'image', 'fashion', 'event', 'work', 'official', 'topic', 'internal', 'other'
));

ALTER TABLE contents ADD COLUMN origin_name TEXT NOT NULL DEFAULT '';
ALTER TABLE contents ADD COLUMN origin_platform TEXT NOT NULL DEFAULT '';
ALTER TABLE contents ADD COLUMN origin_type TEXT NOT NULL DEFAULT ''
    CHECK (origin_type IN ('', 'official', 'media', 'community', 'reference', 'internal'));
ALTER TABLE contents ADD COLUMN metadata_status TEXT NOT NULL DEFAULT 'not_attempted'
    CHECK (metadata_status IN ('not_attempted', 'ok', 'failed', 'blocked'));
ALTER TABLE contents ADD COLUMN metadata_error TEXT NOT NULL DEFAULT '';

CREATE INDEX contents_timeline_idx ON contents (
    content_status,
    COALESCE(published_at, discovered_at) DESC,
    created_at DESC,
    id DESC
);
CREATE INDEX contents_discovered_idx ON contents (discovered_at DESC);
CREATE INDEX contents_metadata_status_idx ON contents (metadata_status)
    WHERE metadata_status IN ('failed', 'blocked');

-- ---------------------------------------------------------------------------
-- 搜索服务（可插拔）。API Key 只存环境变量名，密钥值不写入数据库，也不返回前端。
-- ---------------------------------------------------------------------------
CREATE TABLE search_providers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    provider_type   TEXT NOT NULL CHECK (provider_type IN ('bing_rss', 'searxng')),
    endpoint        TEXT NOT NULL DEFAULT '',
    api_key_env     TEXT NOT NULL DEFAULT '',
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    last_test_at    TIMESTAMPTZ,
    last_test_ok    BOOLEAN,
    last_test_error TEXT NOT NULL DEFAULT '',
    last_latency_ms INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- 搜索任务
-- ---------------------------------------------------------------------------
CREATE TABLE search_tasks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL UNIQUE,
    provider_id       UUID NOT NULL REFERENCES search_providers(id) ON DELETE RESTRICT,
    query             TEXT NOT NULL,
    domains           TEXT[] NOT NULL DEFAULT '{}'::text[],
    content_type_hint TEXT NOT NULL DEFAULT '',
    source_type       TEXT NOT NULL DEFAULT 'media'
        CHECK (source_type IN ('official', 'media', 'community', 'reference', 'internal')),
    trust_level       TEXT NOT NULL DEFAULT 'media'
        CHECK (trust_level IN ('official', 'media', 'community', 'reference', 'internal')),
    interval_minutes  INT NOT NULL DEFAULT 360 CHECK (interval_minutes >= 15),
    max_results       INT NOT NULL DEFAULT 20 CHECK (max_results BETWEEN 1 AND 50),
    enabled           BOOLEAN NOT NULL DEFAULT true,
    last_run_at       TIMESTAMPTZ,
    last_success_at   TIMESTAMPTZ,
    last_failure_at   TIMESTAMPTZ,
    last_error        TEXT NOT NULL DEFAULT '',
    next_run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX search_tasks_due_idx ON search_tasks (next_run_at) WHERE enabled = true;

ALTER TABLE contents ADD COLUMN search_task_id UUID REFERENCES search_tasks(id) ON DELETE SET NULL;
CREATE INDEX contents_search_task_idx ON contents (search_task_id, created_at DESC)
    WHERE search_task_id IS NOT NULL;

CREATE TABLE search_runs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id        UUID NOT NULL REFERENCES search_tasks(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'success', 'failed')),
    trigger_type   TEXT NOT NULL DEFAULT 'schedule'
        CHECK (trigger_type IN ('schedule', 'manual', 'retry')),
    started_at     TIMESTAMPTZ,
    finished_at    TIMESTAMPTZ,
    fetched_count  INT NOT NULL DEFAULT 0,
    inserted_count INT NOT NULL DEFAULT 0,
    duplicate_count INT NOT NULL DEFAULT 0,
    failed_count   INT NOT NULL DEFAULT 0,
    parsed_ok_count INT NOT NULL DEFAULT 0,
    parse_failed_count INT NOT NULL DEFAULT 0,
    error_message  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX search_runs_task_created_idx ON search_runs (task_id, created_at DESC);
CREATE UNIQUE INDEX search_runs_one_active_idx ON search_runs (task_id)
    WHERE status IN ('queued', 'running');

-- ---------------------------------------------------------------------------
-- 基础事件关联。事件只做聚合展示，不改变主时间线排序。
-- ---------------------------------------------------------------------------
CREATE TABLE events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artist_id   UUID NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    summary     TEXT NOT NULL DEFAULT '',
    cover_url   TEXT NOT NULL DEFAULT '',
    event_date  DATE,
    location    TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX events_artist_date_idx ON events (artist_id, event_date DESC NULLS LAST);
CREATE INDEX events_public_idx ON events (status, event_date DESC NULLS LAST);

CREATE TABLE content_events (
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    event_id   UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, event_id)
);
CREATE INDEX content_events_event_idx ON content_events (event_id);
