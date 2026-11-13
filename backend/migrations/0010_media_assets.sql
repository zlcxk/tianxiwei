-- Curated media assets bundled with the frontend.
-- Files live under frontend/public/media and are served from /media/... at runtime.
-- Every statement only fills an empty field, so administrator uploads and later
-- edits are never overwritten when migrations run again.

-- ---------------------------------------------------------------------------
-- 艺人头像与封面
-- ---------------------------------------------------------------------------
UPDATE artists
SET avatar = '/media/artist/avatar.jpg', updated_at = now()
WHERE name = '田曦薇' AND avatar = '';

UPDATE artists
SET cover = '/media/artist/cover.jpg', updated_at = now()
WHERE name = '田曦薇' AND cover = '';

-- ---------------------------------------------------------------------------
-- 作品封面（3:4，900×1200）
-- ---------------------------------------------------------------------------
UPDATE works w
SET cover = v.cover, updated_at = now()
FROM (
    VALUES
        ('在悠长的时光里等你', '/media/works/w16-youzhang.jpg'),
        ('我的真朋友', '/media/works/w01-zhenpengyou.jpg'),
        ('锦绣南歌', '/media/works/w02-jinxiunange.jpg'),
        ('如此可爱的我们', '/media/works/w03-rucikeai.jpg'),
        ('我的宠物少将军', '/media/works/w04-chongwu.jpg'),
        ('我的邻居长不大', '/media/works/w05-linju.jpg'),
        ('卿卿日常', '/media/works/w06-qingqing.jpg'),
        ('初次爱你', '/media/works/w07-chuciaini.jpg'),
        ('花轿喜事', '/media/works/w08-huajiao.jpg'),
        ('田耕纪', '/media/works/w09-tiangengji.jpg'),
        ('半熟男女', '/media/works/w10-banshu.jpg'),
        ('大奉打更人', '/media/works/w11-dafeng.jpg'),
        ('子夜归', '/media/works/w12-ziyegui.jpg'),
        ('逐玉', '/media/works/w13-zhuyu.jpg'),
        ('低智商犯罪', '/media/works/w14-dizhishang.jpg'),
        ('嫁金钗', '/media/works/w15-jiajinchai.jpg'),
        ('天才，女友', '/media/works/w17-tiancainvyou.jpg'),
        ('哈尔的移动城堡', '/media/works/w18-haer.jpg'),
        ('青风吹又生', '/media/works/w19-qingfeng.jpg'),
        ('忙忙碌碌寻宝藏·双人成行季', '/media/works/w20-mangmang.jpg')
) AS v(title, cover)
WHERE w.title = v.title
  AND w.artist_id = (SELECT id FROM artists WHERE name = '田曦薇')
  AND w.cover = '';

-- ---------------------------------------------------------------------------
-- 内容封面。作品/角色档案复用对应作品封面，其余使用独立图片。
-- 没有可核实图片的内容保持留空，由前端抽象封面兜底。
-- ---------------------------------------------------------------------------
UPDATE contents c
SET cover_url = v.cover, updated_at = now()
FROM (
    VALUES
        ('角色档案｜《卿卿日常》李薇', '/media/works/w06-qingqing.jpg'),
        ('角色档案｜《田耕纪》连蔓儿', '/media/works/w09-tiangengji.jpg'),
        ('角色档案｜《大奉打更人》临安公主', '/media/works/w11-dafeng.jpg'),
        ('角色档案｜《子夜归》武祯', '/media/works/w12-ziyegui.jpg'),
        ('角色档案｜《逐玉》樊长玉', '/media/works/w13-zhuyu.jpg'),
        ('角色档案｜《低智商犯罪》李茜', '/media/works/w14-dizhishang.jpg'),
        ('角色档案｜《嫁金钗》魏娆与君黛', '/media/works/w15-jiajinchai.jpg'),
        ('演艺笔记｜从《如此可爱的我们》到《嫁金钗》', '/media/contents/timeline.jpg'),
        ('角色档案｜《花轿喜事》李玉湖', '/media/works/w08-huajiao.jpg'),
        ('角色档案｜《半熟男女》何知南', '/media/works/w10-banshu.jpg'),
        ('作品档案｜《在悠长的时光里等你》：首部影视作品', '/media/works/w16-youzhang.jpg'),
        ('作品档案｜《如此可爱的我们》：黄橙子与青春群像', '/media/works/w03-rucikeai.jpg'),
        ('作品档案｜《哈尔的移动城堡》：首次动画电影配音', '/media/works/w18-haer.jpg'),
        ('作品档案｜《逐玉》：樊长玉的成长线', '/media/works/w13-zhuyu.jpg'),
        ('作品档案｜《低智商犯罪》：李茜与悬疑犯罪题材', '/media/works/w14-dizhishang.jpg'),
        ('作品档案｜《天才，女友》：林知夏的天才设定', '/media/works/w17-tiancainvyou.jpg'),
        ('封面记录｜GRAZIA 新加坡 / 马来西亚九月刊双封面', '/media/contents/grazia.jpg'),
        ('综艺档案｜《忙忙碌碌寻宝藏·双人成行季》', '/media/works/w20-mangmang.jpg'),
        ('生日档案｜1997 年 10 月 14 日，重庆', '/media/contents/birthday.jpg'),
        ('成长档案｜从重庆到上海戏剧学院', '/media/contents/growth.jpg'),
        ('品牌记录｜Gucci 品牌代言人官宣', '/media/contents/gucci.jpg'),
        ('年度回顾｜2026 年的作品与角色版图', '/media/contents/2026-review.jpg'),
        ('活动记录｜《嫁金钗》开播', '/media/works/w15-jiajinchai.jpg'),
        ('活动记录｜《天才，女友》开播', '/media/works/w17-tiancainvyou.jpg')
) AS v(title, cover)
WHERE c.title = v.title
  AND c.artist_id = (SELECT id FROM artists WHERE name = '田曦薇')
  AND c.cover_url = '';

-- ---------------------------------------------------------------------------
-- 角色头像。只使用能确认是该作品角色剧照的图片；其余角色保持留白。
-- ---------------------------------------------------------------------------
UPDATE characters ch
SET avatar = v.avatar
FROM (
    VALUES
        ('锦绣南歌', '陆婉', '/media/characters/luwan.jpg'),
        ('卿卿日常', '李薇', '/media/characters/liwei.jpg'),
        ('初次爱你', '卢晚晚', '/media/characters/luwanwan.jpg'),
        ('花轿喜事', '李玉湖', '/media/characters/liyuhu.jpg'),
        ('半熟男女', '何知南', '/media/characters/hezhinan.jpg'),
        ('大奉打更人', '临安公主', '/media/characters/linan.jpg'),
        ('子夜归', '武祯', '/media/characters/wuzhen.jpg'),
        ('逐玉', '樊长玉', '/media/characters/fanchangyu.jpg'),
        ('低智商犯罪', '李茜', '/media/characters/liqian.jpg')
) AS v(work_title, name, avatar)
WHERE ch.work_id = (
        SELECT w.id
        FROM works w
        WHERE w.title = v.work_title
          AND w.artist_id = (SELECT id FROM artists WHERE name = '田曦薇')
    )
  AND ch.name = v.name
  AND ch.avatar = '';
