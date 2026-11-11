-- Editorial picks: let administrators curate published content on the home page.
-- Picks reference existing contents; no duplicated content model and no image files.
CREATE TABLE IF NOT EXISTS editorial_picks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT editorial_picks_window_check
        CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);

-- One pick per content, so the same item cannot appear twice on the home page.
CREATE UNIQUE INDEX IF NOT EXISTS editorial_picks_content_uidx
    ON editorial_picks (content_id);

CREATE INDEX IF NOT EXISTS editorial_picks_active_idx
    ON editorial_picks (enabled, sort_order, created_at DESC);
