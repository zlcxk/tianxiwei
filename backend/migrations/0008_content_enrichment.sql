-- 内容资料补充（V1.3）。
--
-- 原则：
-- 1. 只新增经核验的缺失作品、角色、事件与关联关系；
-- 2. 只填充空字段，或在 jsonb profile_data 中补充尚不存在的键，
--    绝不覆盖管理员已完善的内容；
-- 3. 不写入任何图片 URL，头像、封面由管理员通过后台上传；
-- 4. 事件没有唯一标题约束，使用 NOT EXISTS 保证幂等；
-- 5. 不修改内容的发布时间、审核状态、可信等级与主时间线顺序。

-- ---------------------------------------------------------------------------
-- 艺人资料：仅补充 profile_data 中不存在的键。现有键（出生日期、出生地、
-- 院校、职业、出道年份、来源等）保持原值。
-- ---------------------------------------------------------------------------
UPDATE artists
SET profile_data = jsonb_build_object(
        'career_overview',
        '2018 年出演《在悠长的时光里等你》进入演艺行业，随后通过《如此可爱的我们》《卿卿日常》《田耕纪》《大奉打更人》等作品积累人气；2026 年《逐玉》《低智商犯罪》《嫁金钗》等剧集相继播出。',
        'debut_work', '在悠长的时光里等你',
        'representative_works', jsonb_build_array('卿卿日常', '田耕纪', '大奉打更人', '逐玉', '嫁金钗')
    ) || profile_data,
    updated_at = now()
WHERE name = '田曦薇'
  AND (
      NOT (profile_data ? 'career_overview')
      OR NOT (profile_data ? 'debut_work')
      OR NOT (profile_data ? 'representative_works')
  );

-- ---------------------------------------------------------------------------
-- 缺失作品。作品唯一索引为 (artist_id, title)，重复执行不会产生重复记录。
-- 已有作品的简介、日期、平台、状态不做覆盖。
-- ---------------------------------------------------------------------------
INSERT INTO works (artist_id, title, type, description, release_date, status, platform, official_url)
SELECT a.id, v.title, v.type, v.description, v.release_date, v.status, v.platform, v.official_url
FROM artists a
JOIN (
    VALUES
        ('在悠长的时光里等你', 'drama',
         '都市情感剧。田曦薇在剧中饰演骆熙，这是她进入演艺行业的首部作品。',
         DATE '2018-07-13', 'aired', '爱奇艺', 'https://baike.baidu.com/item/在悠长的时光里等你'),
        ('天才，女友', 'drama',
         '都市爱情剧。田曦薇在剧中饰演 IQ 174 的数学物理天才林知夏，与胡一天搭档主演。',
         DATE '2026-08-02', 'aired', '爱奇艺', 'https://baike.baidu.com/item/天才，女友'),
        ('哈尔的移动城堡', 'movie',
         '宫崎骏动画电影中文配音版。田曦薇为女主角苏菲配音，这是她首次参与动画电影配音。',
         DATE '2024-04-30', 'aired', '全国院线', 'https://baike.baidu.com/item/哈尔的移动城堡'),
        ('青风吹又生', 'drama',
         '都市题材剧集。田曦薇在剧中饰演林青；具体播出时间以官方发布为准。',
         NULL, 'upcoming', '', 'https://baike.baidu.com/item/青风吹又生'),
        ('忙忙碌碌寻宝藏·双人成行季', 'variety',
         '芒果 TV 户外寻宝综艺。田曦薇作为常驻嘉宾参与本季节目。',
         DATE '2026-06-21', 'aired', '芒果TV', 'https://baike.baidu.com/item/忙忙碌碌寻宝藏')
) AS v(title, type, description, release_date, status, platform, official_url)
    ON true
WHERE a.name = '田曦薇'
ON CONFLICT (artist_id, title) DO NOTHING;

-- 仅补充空的播出平台，已有平台保持原值。
UPDATE works w
SET platform = v.platform, updated_at = now()
FROM artists a
JOIN (
    VALUES
        ('半熟男女', '优酷'),
        ('子夜归', '腾讯视频')
) AS v(title, platform) ON true
WHERE w.artist_id = a.id
  AND a.name = '田曦薇'
  AND w.title = v.title
  AND w.platform = '';

-- ---------------------------------------------------------------------------
-- 新作品的缺失角色。角色唯一索引为 (work_id, name)。
-- ---------------------------------------------------------------------------
INSERT INTO characters (work_id, name, description)
SELECT w.id, v.name, v.description
FROM works w
JOIN artists a ON a.id = w.artist_id
JOIN (
    VALUES
        ('在悠长的时光里等你', '骆熙', '田曦薇首个影视角色，都市情感故事中的年轻女性。'),
        ('天才，女友', '林知夏', 'IQ 174 的数学物理天才，故事围绕她与男主角的关系展开。'),
        ('哈尔的移动城堡', '苏菲', '动画电影女主角，田曦薇在中文配音版中为苏菲配音。'),
        ('青风吹又生', '林青', '都市剧中的女主角；角色资料以官方发布为准。')
) AS v(work_title, name, description) ON v.work_title = w.title
WHERE a.name = '田曦薇'
ON CONFLICT (work_id, name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 真实公开活动 / 作品节点。事件只做聚合，不改变内容时间线。
-- 标题相同的事件不会重复插入；封面留空，由管理员上传或填写。
-- ---------------------------------------------------------------------------
INSERT INTO events (artist_id, title, summary, event_date, location, status)
SELECT a.id, v.title, v.summary, v.event_date, v.location, 'published'
FROM artists a
JOIN (
    VALUES
        ('2026 央视春节联欢晚会',
         '参加中央广播电视总台《2026 年春节联欢晚会》，参与表演节目《串门指南》。',
         DATE '2026-02-16', '北京'),
        ('《逐玉》开播',
         '古装爱情剧《逐玉》开播，田曦薇饰演樊长玉。具体播出信息以平台官方为准。',
         DATE '2026-03-06', '腾讯视频 / 爱奇艺'),
        ('《你好，星期六》逐玉宣传',
         '参与湖南卫视《你好，星期六》节目录制，为《逐玉》进行宣传。',
         DATE '2026-03-07', '长沙'),
        ('《时装男士》六月刊封面发布',
         '首登《时装男士 L''OFFICIEL HOMMES》六月刊，以三封面形式呈现，由 Gucci 与周生生支持。',
         DATE '2026-06-13', ''),
        ('《忙忙碌碌寻宝藏·双人成行季》播出',
         '作为常驻嘉宾参与的芒果 TV 户外寻宝综艺《忙忙碌碌寻宝藏·双人成行季》播出。',
         DATE '2026-06-21', '芒果TV'),
        ('釜山亚洲内容大奖',
         '亮相釜山亚洲内容大奖，凭《逐玉》获得最佳女主角提名，并获人气奖。',
         DATE '2026-06-20', '釜山'),
        ('《天才，女友》开播',
         '都市爱情剧《天才，女友》开播，田曦薇饰演数学物理天才林知夏。',
         DATE '2026-08-02', '爱奇艺'),
        ('GRAZIA 新加坡 / 马来西亚九月刊双封面',
         '登上《红秀 GRAZIA》新加坡版与马来西亚版九月刊封面，造型来自 Gucci 新季系列。',
         DATE '2026-09-01', '新加坡 / 马来西亚'),
        ('Gucci 品牌代言人官宣',
         'Gucci 宣布田曦薇为品牌代言人，后续出席米兰时装周等活动。',
         DATE '2026-09-21', ''),
        ('《嫁金钗》开播',
         '古装剧《嫁金钗》开播，田曦薇一人分饰魏娆、君黛两个角色。',
         DATE '2026-10-16', '优酷 / CCTV-8'),
        ('爱奇艺国际版全球代言人官宣',
         '爱奇艺国际版宣布田曦薇为全球代言人。',
         DATE '2026-10-26', ''),
        ('泰国 iJOY TH 2027 发布会',
         '出席在曼谷举行的 iJOY TH 2027 发布会活动。',
         DATE '2026-11-05', '曼谷')
) AS v(title, summary, event_date, location) ON true
WHERE a.name = '田曦薇'
  AND NOT EXISTS (
      SELECT 1 FROM events e
      WHERE e.artist_id = a.id AND e.title = v.title
  );
