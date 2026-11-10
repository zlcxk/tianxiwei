CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    nickname      TEXT NOT NULL DEFAULT '',
    avatar        TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'editor', 'user')),
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_uidx ON users (lower(email));

CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    ip         TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE artists (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    name_en      TEXT NOT NULL DEFAULT '',
    avatar       TEXT NOT NULL DEFAULT '',
    cover        TEXT NOT NULL DEFAULT '',
    bio          TEXT NOT NULL DEFAULT '',
    profile_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX artists_name_uidx ON artists (name);

CREATE TABLE works (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artist_id    UUID NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    type         TEXT NOT NULL DEFAULT 'drama' CHECK (type IN ('drama', 'movie', 'variety', 'other')),
    cover        TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    release_date DATE,
    status       TEXT NOT NULL DEFAULT 'aired' CHECK (status IN ('aired', 'airing', 'upcoming')),
    platform     TEXT NOT NULL DEFAULT '',
    official_url TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX works_artist_title_uidx ON works (artist_id, title);
CREATE INDEX works_artist_release_idx ON works (artist_id, release_date DESC NULLS LAST);

CREATE TABLE characters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_id     UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    avatar      TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX characters_work_name_uidx ON characters (work_id, name);

CREATE TABLE scenes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',
    sort_order  INT NOT NULL DEFAULT 0,
    enabled     BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE sources (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL UNIQUE,
    platform          TEXT NOT NULL DEFAULT '',
    source_type       TEXT NOT NULL DEFAULT 'media' CHECK (source_type IN ('official', 'media', 'community', 'reference', 'internal')),
    adapter_type      TEXT NOT NULL DEFAULT 'manual' CHECK (adapter_type IN ('rss', 'manual')),
    config            JSONB NOT NULL DEFAULT '{}'::jsonb,
    trust_level       TEXT NOT NULL DEFAULT 'media' CHECK (trust_level IN ('official', 'media', 'community', 'reference', 'internal')),
    enabled           BOOLEAN NOT NULL DEFAULT true,
    interval_minutes  INT NOT NULL DEFAULT 120 CHECK (interval_minutes >= 5),
    last_success_at   TIMESTAMPTZ,
    last_failure_at   TIMESTAMPTZ,
    last_error        TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE contents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artist_id       UUID NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    source_id       UUID REFERENCES sources(id) ON DELETE SET NULL,
    source_item_id  TEXT,
    title           TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    content_type    TEXT NOT NULL DEFAULT 'other' CHECK (content_type IN ('social', 'drama', 'variety', 'media', 'topic', 'image', 'fashion', 'event', 'internal', 'other')),
    source_author   TEXT NOT NULL DEFAULT '',
    source_url      TEXT NOT NULL DEFAULT '',
    cover_url       TEXT NOT NULL DEFAULT '',
    published_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    popularity      JSONB NOT NULL DEFAULT '{}'::jsonb,
    trust_level     TEXT NOT NULL DEFAULT 'media' CHECK (trust_level IN ('official', 'media', 'community', 'reference', 'internal')),
    is_official     BOOLEAN NOT NULL DEFAULT false,
    review_status   TEXT NOT NULL DEFAULT 'none' CHECK (review_status IN ('none', 'pending', 'approved', 'rejected')),
    content_status  TEXT NOT NULL DEFAULT 'pending' CHECK (content_status IN ('pending', 'reviewing', 'published', 'rejected', 'expired', 'archived')),
    review_reason   TEXT NOT NULL DEFAULT '',
    url_hash        TEXT,
    content_hash    TEXT,
    dedupe_of       UUID REFERENCES contents(id) ON DELETE SET NULL,
    raw_data        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX contents_source_item_uidx ON contents (source_id, source_item_id)
    WHERE source_item_id IS NOT NULL AND source_item_id <> '';
CREATE UNIQUE INDEX contents_url_hash_uidx ON contents (url_hash)
    WHERE url_hash IS NOT NULL AND url_hash <> '';
CREATE INDEX contents_public_idx ON contents (content_status, published_at DESC);
CREATE INDEX contents_review_idx ON contents (review_status, created_at DESC) WHERE review_status = 'pending';
CREATE INDEX contents_hash_idx ON contents (content_hash);
CREATE INDEX contents_artist_created_idx ON contents (artist_id, created_at DESC);

CREATE TABLE content_scenes (
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    scene_id   UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, scene_id)
);
CREATE INDEX content_scenes_scene_idx ON content_scenes (scene_id);

CREATE TABLE content_works (
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    work_id    UUID NOT NULL REFERENCES works(id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, work_id)
);
CREATE INDEX content_works_work_idx ON content_works (work_id);

CREATE TABLE content_characters (
    content_id   UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, character_id)
);
CREATE INDEX content_characters_character_idx ON content_characters (character_id);

CREATE TABLE user_scene_preferences (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scene_id   UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    weight     INT NOT NULL DEFAULT 3 CHECK (weight BETWEEN 1 AND 5),
    enabled    BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, scene_id)
);
CREATE UNIQUE INDEX user_scene_one_default_idx ON user_scene_preferences (user_id) WHERE is_default = true;

CREATE TABLE favorites (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('content', 'work', 'character')),
    target_id   UUID NOT NULL,
    snapshot    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX favorites_user_target_uidx ON favorites (user_id, target_type, target_id);
CREATE INDEX favorites_user_created_idx ON favorites (user_id, created_at DESC);

CREATE TABLE collection_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id       UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'success', 'failed', 'skipped')),
    trigger_type    TEXT NOT NULL DEFAULT 'schedule' CHECK (trigger_type IN ('schedule', 'manual', 'retry')),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    fetched_count   INT NOT NULL DEFAULT 0,
    inserted_count  INT NOT NULL DEFAULT 0,
    duplicate_count INT NOT NULL DEFAULT 0,
    failed_count    INT NOT NULL DEFAULT 0,
    error_message   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX collection_jobs_source_created_idx ON collection_jobs (source_id, created_at DESC);
CREATE UNIQUE INDEX collection_jobs_one_active_idx ON collection_jobs (source_id)
    WHERE status IN ('queued', 'running');

CREATE TABLE review_logs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id UUID REFERENCES contents(id) ON DELETE SET NULL,
    user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    action     TEXT NOT NULL,
    before     JSONB,
    after      JSONB,
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX review_logs_content_idx ON review_logs (content_id, created_at DESC);

CREATE TABLE classification_rules (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scene_id   UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    keywords   TEXT NOT NULL,
    priority   INT NOT NULL DEFAULT 0,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (scene_id, keywords)
);
CREATE INDEX classification_rules_enabled_idx ON classification_rules (enabled, priority DESC);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
