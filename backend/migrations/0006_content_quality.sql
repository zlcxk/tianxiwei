-- ---------------------------------------------------------------------------
-- 内容质量治理：人物相关性评分、自动过滤原因、搜索关键词收敛。
--
-- 迁移保持向后兼容：历史内容 relevance_score=0、relevance_label=''、
-- filter_reason=''，行为与迁移前一致；被自动过滤的内容复用现有
-- content_status/review_status='rejected'，后台可通过「重新审核」恢复。
-- ---------------------------------------------------------------------------

ALTER TABLE contents
    ADD COLUMN IF NOT EXISTS relevance_score INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS relevance_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS filter_reason TEXT NOT NULL DEFAULT '';

ALTER TABLE contents DROP CONSTRAINT IF EXISTS contents_relevance_label_check;
ALTER TABLE contents ADD CONSTRAINT contents_relevance_label_check
    CHECK (relevance_label IN ('', 'high', 'candidate', 'filtered'));

ALTER TABLE contents DROP CONSTRAINT IF EXISTS contents_filter_reason_check;
ALTER TABLE contents ADD CONSTRAINT contents_filter_reason_check
    CHECK (filter_reason IN (
        '', 'irrelevant_person', 'ambiguous_entity', 'dictionary_content',
        'unrelated_person', 'insufficient_context', 'duplicate_content',
        'invalid_source'
    ));

CREATE INDEX IF NOT EXISTS contents_filter_reason_idx
    ON contents (filter_reason, discovered_at DESC)
    WHERE filter_reason <> '';

CREATE INDEX IF NOT EXISTS contents_relevance_idx
    ON contents (relevance_label, discovered_at DESC)
    WHERE search_task_id IS NOT NULL AND relevance_label <> '';

-- 人物实体配置。别名必须经过人工核验后由管理员补充，默认留空。
INSERT INTO settings (key, value) VALUES (
    'artist_entity',
    '{
        "name": "田曦薇",
        "aliases": [],
        "verified_accounts": [],
        "excluded_terms": [
            "田姓", "田姓氏", "田字解释", "汉字田", "田字组词",
            "田的部首", "田的拼音", "田的笔画", "田字成语", "百家姓"
        ]
    }'::jsonb
)
ON CONFLICT (key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 搜索任务：只允许“人物精确 + 组合”查询，不再保留泛关键词任务。
-- 仅更新仍为初始默认值的任务，避免覆盖管理员已经调整过的配置。
-- ---------------------------------------------------------------------------
UPDATE search_tasks
SET query = '田曦薇 作品 采访', updated_at = now()
WHERE name = '田曦薇 · 作品与采访' AND query = '田曦薇 新剧 采访';

INSERT INTO search_tasks (name, provider_id, query, domains, content_type_hint,
    source_type, trust_level, interval_minutes, max_results, enabled, next_run_at)
SELECT v.name, p.id, v.query, '{}'::text[], v.content_type_hint, 'community', 'community',
       720, 15, true, now() + (v.delay || ' minutes')::interval
FROM search_providers p
CROSS JOIN (VALUES
    ('田曦薇 · 微博动态', '田曦薇 微博', 'post', 15),
    ('田曦薇 · 小红书动态', '田曦薇 小红书', 'post', 30),
    ('田曦薇 · 活动与品牌', '田曦薇 活动 品牌', 'event', 45),
    ('田曦薇 · 写真与花絮', '田曦薇 写真 花絮', 'image', 60)
) AS v(name, query, content_type_hint, delay)
WHERE p.name = '必应网页搜索 RSS'
ON CONFLICT (name) DO NOTHING;
