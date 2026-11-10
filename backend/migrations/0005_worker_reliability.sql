-- ---------------------------------------------------------------------------
-- Worker 调度可靠性：任务心跳、partial 终态与领取索引。
--
-- heartbeat_at 由执行中的 Worker 每 30 秒刷新；超时回收以
-- COALESCE(heartbeat_at, started_at) 为准，避免误回收仍在执行的长任务。
-- ---------------------------------------------------------------------------

ALTER TABLE collection_jobs ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ;
ALTER TABLE search_runs ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ;

ALTER TABLE collection_jobs DROP CONSTRAINT IF EXISTS collection_jobs_status_check;
ALTER TABLE collection_jobs ADD CONSTRAINT collection_jobs_status_check
    CHECK (status IN ('queued', 'running', 'success', 'partial', 'failed', 'skipped'));

ALTER TABLE search_runs DROP CONSTRAINT IF EXISTS search_runs_status_check;
ALTER TABLE search_runs ADD CONSTRAINT search_runs_status_check
    CHECK (status IN ('queued', 'running', 'success', 'partial', 'failed'));

CREATE INDEX IF NOT EXISTS collection_jobs_queued_idx
    ON collection_jobs (created_at) WHERE status = 'queued';
CREATE INDEX IF NOT EXISTS search_runs_queued_idx
    ON search_runs (created_at) WHERE status = 'queued';
