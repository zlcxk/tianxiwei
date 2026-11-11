package discovery

import "testing"

func TestIdentifySourceUsesHostBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		wantName  string
		wantType  string
		wantKnown bool
	}{
		{"weibo root", "https://weibo.com/u/123", "微博", "community", true},
		{"weibo subdomain", "https://m.weibo.cn/status/1", "微博", "community", true},
		{"weibo spoof prefix", "https://fakeweibo.com/u/123", "fakeweibo.com", "media", false},
		{"weibo spoof suffix", "https://weibo.com.attacker.tld/u/123", "weibo.com.attacker.tld", "media", false},
		{"weibo dot suffix lookalike", "https://notweibo.cn.example.org", "notweibo.cn.example.org", "media", false},
		{"weibo uppercase port dot", "https://WEIBO.COM.:443/u/123", "微博", "community", true},
		{"xiaohongshu", "https://www.xiaohongshu.com/explore/x", "小红书", "community", true},
		{"douyin", "https://v.douyin.com/x", "抖音", "community", true},
		{"bilibili", "https://space.bilibili.com/1", "哔哩哔哩", "community", true},
		{"zhihu", "https://zhuanlan.zhihu.com/p/1", "知乎", "community", true},
		{"baike", "https://baike.baidu.com/item/田曦薇", "百科资料", "reference", true},
		{"wikipedia", "https://zh.wikipedia.org/wiki/x", "百科资料", "reference", true},
		{"wechat article", "https://mp.weixin.qq.com/s/x", "微信公众号", "media", true},
		{"wechat root", "https://weixin.qq.com/x", "微信公众号", "media", true},
		{"tencent qq", "https://news.qq.com/a", "腾讯新闻", "media", true},
		{"qq spoof", "https://qq.com.evil.tld/a", "qq.com.evil.tld", "media", false},
		{"sohu", "https://www.sohu.com/a", "搜狐", "media", true},
		{"sina cn", "https://ent.sina.cn/x", "新浪", "media", true},
		{"netease", "https://ent.163.com/x", "网易", "media", true},
		{"ifeng", "https://ent.ifeng.com/x", "凤凰网", "media", true},
		{"thepaper", "https://www.thepaper.cn/x", "澎湃新闻", "media", true},
		{"cctv", "https://www.cctv.com/x", "官方媒体", "official", true},
		{"cctv spoof", "https://fakecctv.com/x", "fakecctv.com", "media", false},
		{"cnr", "https://www.cnr.cn/x", "官方媒体", "official", true},
		{"xinhuanet", "https://www.xinhuanet.com/x", "官方媒体", "official", true},
		{"xinhuanet spoof", "https://fakexinhuanet.com/x", "fakexinhuanet.com", "media", false},
		{"xinhuanet suffix spoof", "https://xinhuanet.com.evil.tld/x", "xinhuanet.com.evil.tld", "media", false},
		{"people", "https://people.com.cn/x", "官方媒体", "official", true},
		{"baidu", "https://www.baidu.com/s?wd=x", "百度", "reference", true},
		{"baidu spoof", "https://baidu.com.cn/x", "baidu.com.cn", "media", false},
		{"unknown", "https://example.org/post", "example.org", "media", false},
		{"empty", "", "", "media", false},
		{"invalid", "not a url", "", "media", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IdentifySource(tc.url)
			if got.Name != tc.wantName || got.SourceType != tc.wantType || got.Known != tc.wantKnown {
				t.Fatalf("IdentifySource(%q) = %+v, want name=%q type=%q known=%v",
					tc.url, got, tc.wantName, tc.wantType, tc.wantKnown)
			}
		})
	}
}

func TestPlatformForURLMatchesIdentifySource(t *testing.T) {
	name, platform, sourceType := PlatformForURL("https://m.weibo.cn/status/1")
	if name != "微博" || platform != "微博" || sourceType != "community" {
		t.Fatalf("PlatformForURL = %q/%q/%q", name, platform, sourceType)
	}
}
