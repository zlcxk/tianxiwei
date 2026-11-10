-- 默认搜索服务与任务。仅包含无需密钥、公开可访问的 Bing Web Search RSS。
-- 搜索结果仍需人工审核后才会公开；任务可在管理端停用或替换。
INSERT INTO search_providers (name, provider_type, endpoint, api_key_env, config, enabled)
VALUES ('必应网页搜索 RSS', 'bing_rss', 'https://www.bing.com/search', '', '{"locale":"zh-Hans"}'::jsonb, true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO search_tasks (name, provider_id, query, domains, content_type_hint,
    source_type, trust_level, interval_minutes, max_results, enabled, next_run_at)
SELECT '田曦薇 · 综合资讯', p.id, '田曦薇', '{}'::text[], '', 'media', 'media', 360, 20, true, now()
FROM search_providers p WHERE p.name='必应网页搜索 RSS'
ON CONFLICT (name) DO NOTHING;

INSERT INTO search_tasks (name, provider_id, query, domains, content_type_hint,
    source_type, trust_level, interval_minutes, max_results, enabled, next_run_at)
SELECT '田曦薇 · 作品与采访', p.id, '田曦薇 新剧 采访', '{}'::text[], '', 'media', 'media', 720, 15, true, now() + interval '10 minutes'
FROM search_providers p WHERE p.name='必应网页搜索 RSS'
ON CONFLICT (name) DO NOTHING;
