package collector

import (
	"strings"
	"testing"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

func TestStripHTMLRemovesMarkupAndScripts(t *testing.T) {
	input := `<p>田曦薇 <strong>新剧</strong> 开机</p><script>alert("x")</script><style>.x{}</style>`
	got := stripHTML(input)
	if got != "田曦薇 新剧 开机" {
		t.Fatalf("stripHTML = %q", got)
	}
	if stripHTML("  ") != "" {
		t.Fatal("blank input should produce blank output")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("田曦薇新剧开机", 3); got != "田曦薇…" {
		t.Fatalf("truncateRunes = %q", got)
	}
	if got := truncateRunes("短", 5); got != "短" {
		t.Fatalf("short string should be unchanged, got %q", got)
	}
}

func TestCanonicalURL(t *testing.T) {
	cases := map[string]string{
		"HTTPS://Example.COM/path/?utm_source=x&utm_medium=y&id=7#frag": "https://example.com/path?id=7",
		"https://example.com/path/":                                     "https://example.com/path",
		"https://example.com/?spm=abc&from=weibo":                       "https://example.com/",
		"":          "",
		"not a url": "not a url",
	}
	for input, want := range cases {
		if got := canonicalURL(input); got != want {
			t.Errorf("canonicalURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestContentFingerprintNormalizesWhitespace(t *testing.T) {
	a := contentFingerprint("  田曦薇  新剧 ", "今日\n开机")
	b := contentFingerprint("田曦薇 新剧", "今日 开机")
	if a != b {
		t.Fatal("fingerprint should ignore surrounding and repeated whitespace")
	}
	if contentFingerprint("标题一", "正文") == contentFingerprint("标题二", "正文") {
		t.Fatal("different content must produce different fingerprints")
	}
}

func TestSplitKeywords(t *testing.T) {
	got := splitKeywords("剧集, 综艺，代言、红毯; 开机\n写真")
	want := []string{"剧集", "综艺", "代言", "红毯", "开机", "写真"}
	if len(got) != len(want) {
		t.Fatalf("splitKeywords = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitKeywords[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestClassify(t *testing.T) {
	rules := []store.Rule{
		{SceneID: "s1", SceneName: "剧粉", Keywords: "剧集,开播", Enabled: true},
		{SceneID: "s2", SceneName: "综艺粉", Keywords: "综艺，节目", Enabled: true},
		{SceneID: "s3", SceneName: "颜值粉", Keywords: "写真", Enabled: false},
	}
	got := Classify("新剧《卿卿日常》即将开播，她也参加了一档综艺节目。", rules)
	if len(got.SceneIDs) != 2 {
		t.Fatalf("expected 2 scenes, got %#v", got)
	}
	if got.Scenes[0] != "剧粉" || got.Scenes[1] != "综艺粉" {
		t.Fatalf("unexpected scene names: %#v", got.Scenes)
	}
	if got := Classify("一则写真新闻", rules); len(got.SceneIDs) != 0 {
		t.Fatalf("disabled rules must not match: %#v", got.SceneIDs)
	}
}

func TestDecideReview(t *testing.T) {
	media := models.Source{SourceType: "media", TrustLevel: "media"}
	if d := DecideReview(media, "新剧正式官宣定档", 2); d.ReviewStatus != "pending" || !strings.Contains(d.Reason, "官宣") {
		t.Fatalf("important keyword should require review: %#v", d)
	}
	if d := DecideReview(media, "日常分享", 1); d.ReviewStatus != "none" || d.ContentStatus != "published" {
		t.Fatalf("ordinary content should publish: %#v", d)
	}
	if d := DecideReview(models.Source{SourceType: "community", TrustLevel: "community"}, "日常分享", 1); d.ReviewStatus != "pending" {
		t.Fatalf("community content should require review: %#v", d)
	}
	if d := DecideReview(media, "日常分享", 0); d.ReviewStatus != "pending" {
		t.Fatalf("unclassified content should require review: %#v", d)
	}
}

func TestInferContentType(t *testing.T) {
	cases := []struct {
		text   string
		source models.Source
		want   string
	}{
		{"她参加了一档综艺节目", models.Source{SourceType: "media"}, "variety"},
		{"新剧角色海报释出", models.Source{SourceType: "media"}, "drama"},
		{"品牌代言合作官宣", models.Source{SourceType: "official"}, "fashion"},
		{"杂志写真造型", models.Source{SourceType: "media"}, "image"},
		{"出席发布会红毯", models.Source{SourceType: "media"}, "event"},
		{"无关内容", models.Source{SourceType: "official"}, "social"},
		{"无关内容", models.Source{SourceType: "media"}, "media"},
		{"无关内容", models.Source{SourceType: "community"}, "other"},
	}
	for _, c := range cases {
		if got := InferContentType(c.source, c.text); got != c.want {
			t.Errorf("InferContentType(%q, %s) = %q, want %q", c.text, c.source.SourceType, got, c.want)
		}
	}
}
