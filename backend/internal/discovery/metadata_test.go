package discovery

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseBingDate(t *testing.T) {
	got := parseBingDate("周一, 09 11月 2026 22:02:00 GMT")
	if got == nil {
		t.Fatal("expected Chinese RSS date to parse")
	}
	want := time.Date(2026, 11, 9, 22, 2, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parsed = %v, want %v", got, want)
	}
	if parseBingDate("not a date") != nil {
		t.Fatal("invalid date should stay unknown")
	}
}

func TestValidatePublicURL(t *testing.T) {
	blocked := []string{
		"file:///etc/passwd",
		"ftp://example.com/a",
		"http://localhost/a",
		"http://127.0.0.1/a",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://[::1]/",
		"http://0.0.0.0/",
		"http://100.64.0.1/",
		"http://example.com:8080/",
		"http://user:pass@example.com/",
		"http://metadata.google.internal/",
	}
	for _, raw := range blocked {
		if _, err := ValidatePublicURL(raw); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("expected %s to be blocked, got %v", raw, err)
		}
	}
	for _, raw := range []string{"https://example.com/article", "http://www.sohu.com/a/1"} {
		if _, err := ValidatePublicURL(raw); err != nil {
			t.Errorf("expected %s to be allowed, got %v", raw, err)
		}
	}
}

func TestParseHTMLMetadata(t *testing.T) {
	htmlBody := `<!doctype html><html><head>
		<title>Fallback title</title>
		<meta property="og:title" content="田曦薇新剧开机">
		<meta property="og:description" content="官方发布开机消息">
		<meta property="og:site_name" content="示例媒体">
		<meta property="article:published_time" content="2026-11-09T22:02:00+08:00">
		<meta name="author" content="记者甲">
		<link rel="canonical" href="/news/1">
		<script type="application/ld+json">{"@type":"NewsArticle","headline":"备用标题","datePublished":"2026-11-08"}</script>
	</head><body><p>正文内容</p></body></html>`
	got := parseHTMLMetadata(strings.NewReader(htmlBody), "https://example.com/path/page.html")
	if got.Title != "田曦薇新剧开机" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.Description != "官方发布开机消息" {
		t.Fatalf("description = %q", got.Description)
	}
	if got.Author != "记者甲" {
		t.Fatalf("author = %q", got.Author)
	}
	if got.CanonicalURL != "https://example.com/news/1" {
		t.Fatalf("canonical = %q", got.CanonicalURL)
	}
	if got.PublishedAt == nil || got.PublishedAt.Year() != 2026 {
		t.Fatalf("published = %v", got.PublishedAt)
	}
}

func TestInterstitialDetection(t *testing.T) {
	if !looksLikeInterstitial("https://example.com/login", "登录", "") {
		t.Fatal("login URL should be treated as restricted")
	}
	if !looksLikeInterstitial("https://example.com/a", "安全验证", "") {
		t.Fatal("captcha title should be treated as restricted")
	}
	if looksLikeInterstitial("https://example.com/a", "田曦薇新闻", "正文") {
		t.Fatal("normal page should not be restricted")
	}
}

func TestDomainClause(t *testing.T) {
	got := domainClause([]string{"https://weibo.com/u/1", "sohu.com"})
	if got != "(site:weibo.com OR site:sohu.com)" {
		t.Fatalf("domainClause = %q", got)
	}
}

// TestBingLive is an opt-in availability check against the real public endpoint.
func TestBingLive(t *testing.T) {
	if os.Getenv("TEST_LIVE_SEARCH") != "1" {
		t.Skip("set TEST_LIVE_SEARCH=1 to query the live search endpoint")
	}
	provider := NewBingRSSProvider("", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results, err := provider.Search(ctx, SearchRequest{Query: "田曦薇", Page: 1, PageSize: 5})
	if err != nil {
		t.Fatalf("live search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("live search returned zero results")
	}
	for _, r := range results {
		if r.URL == "" || r.Title == "" {
			t.Fatalf("result missing title/url: %#v", r)
		}
	}
	t.Logf("live results: %d, first=%s", len(results), results[0].URL)
}
