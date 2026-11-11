package collector

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xiweispace/backend/internal/discovery"
)

func TestResolveSourceTrust(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		taskTrust string
		want      string
	}{
		{"official media stays official", "https://www.xinhuanet.com/a", "media", "official"},
		{"official media not downgraded by community task", "https://www.cctv.com/a", "community", "official"},
		{"community recognized over official task", "https://weibo.com/u/1", "official", "community"},
		{"reference recognized", "https://baike.baidu.com/item/x", "media", "reference"},
		{"recognized media ignores official task", "https://www.thepaper.cn/a", "official", "media"},
		{"unknown host cannot be promoted to official", "https://example.org/a", "official", "media"},
		{"unknown host accepts conservative fallback", "https://example.org/a", "community", "community"},
		{"unknown host accepts reference fallback", "https://example.org/a", "reference", "reference"},
		{"unknown host defaults to media", "https://example.org/a", "internal", "media"},
		{"spoof host is unknown", "https://fakexinhuanet.com/a", "official", "media"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := discovery.IdentifySource(tc.url)
			if got := resolveSourceTrust(identity, tc.taskTrust); got != tc.want {
				t.Fatalf("resolveSourceTrust(%s, task=%s) = %q, want %q",
					tc.url, tc.taskTrust, got, tc.want)
			}
		})
	}
}

func TestSearchSince(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	if got := searchSince(nil, DefaultSearchOverlap); got != nil {
		t.Fatalf("searchSince(nil) = %v, want nil", got)
	}
	got := searchSince(&now, DefaultSearchOverlap)
	if got == nil {
		t.Fatal("searchSince returned nil for a successful run")
	}
	want := now.Add(-7 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("searchSince = %v, want %v", got, want)
	}
	if DefaultSearchOverlap != 7*24*time.Hour {
		t.Fatalf("DefaultSearchOverlap = %v, want 7 days", DefaultSearchOverlap)
	}
	if got := searchSince(&now, 0); got == nil || !got.Equal(want) {
		t.Fatalf("zero overlap window = %v, want default 7 days", got)
	}
}

func TestResolveSearchCoverURL(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"public https image", "https://cdn.example.com/a.jpg", "https://cdn.example.com/a.jpg"},
		{"public http image", "http://cdn.example.com/a.png", "http://cdn.example.com/a.png"},
		{"trims whitespace", "  https://cdn.example.com/a.webp  ", "https://cdn.example.com/a.webp"},
		{"empty stays empty", "", ""},
		{"rejects javascript scheme", "javascript:alert(1)", ""},
		{"rejects data uri", "data:image/png;base64,AAAA", ""},
		{"rejects localhost", "http://localhost/a.jpg", ""},
		{"rejects loopback ip", "http://127.0.0.1/a.jpg", ""},
		{"rejects internal suffix", "http://printer.internal/a.jpg", ""},
		{"rejects credentials", "https://user:pass@cdn.example.com/a.jpg", ""},
		{"rejects non-standard port", "https://cdn.example.com:8443/a.jpg", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveSearchCoverURL(tc.raw); got != tc.want {
				t.Fatalf("resolveSearchCoverURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestResolveSearchContentType(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		title   string
		summary string
		hint    string
		want    string
	}{
		{"text beats task hint", "https://example.org/a", "田曦薇接受专访", "谈新剧角色", "image", "interview"},
		{"text beats platform default", "https://weibo.com/u/1", "田曦薇新剧开机", "剧组今日开机", "post", "work"},
		{"weibo defaults to post", "https://weibo.com/u/1", "今日公开行程更新", "工作室发布最新行程安排。", "image", "post"},
		{"bilibili defaults to video", "https://www.bilibili.com/video/1", "今日公开行程更新", "工作室发布最新行程安排。", "image", "video"},
		{"official media defaults to news", "https://www.xinhuanet.com/a", "今日公开行程更新", "工作室发布最新行程安排。", "image", "news"},
		{"reference defaults to other", "https://baike.baidu.com/item/x", "今日公开行程更新", "工作室发布最新行程安排。", "image", "other"},
		{"task hint fallback for unknown host", "https://example.org/a", "今日公开行程更新", "工作室发布最新行程安排。", "image", "image"},
		{"unknown host without hint", "https://example.org/a", "今日公开行程更新", "工作室发布最新行程安排。", "", "other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identity := discovery.IdentifySource(tc.url)
			if got := resolveSearchContentType(identity, tc.title, tc.summary, tc.hint); got != tc.want {
				t.Fatalf("resolveSearchContentType = %q, want %q", got, tc.want)
			}
		})
	}
}

type fakePageFetcher struct {
	active int32
	max    int32
	delay  time.Duration
}

func (f *fakePageFetcher) Fetch(ctx context.Context, rawURL string) (discovery.PageMetadata, error) {
	current := atomic.AddInt32(&f.active, 1)
	defer atomic.AddInt32(&f.active, -1)
	for {
		max := atomic.LoadInt32(&f.max)
		if current <= max || atomic.CompareAndSwapInt32(&f.max, max, current) {
			break
		}
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return discovery.PageMetadata{}, ctx.Err()
		}
	}
	switch {
	case strings.Contains(rawURL, "blocked"):
		return discovery.PageMetadata{}, discovery.ErrBlockedResponse
	case strings.Contains(rawURL, "fail"):
		return discovery.PageMetadata{}, errors.New("read failed")
	default:
		return discovery.PageMetadata{Title: rawURL}, nil
	}
}

func TestFetchMetadataBatchIsBoundedAndOrdered(t *testing.T) {
	fetcher := &fakePageFetcher{delay: 10 * time.Millisecond}
	urls := make([]string, 12)
	for i := range urls {
		urls[i] = "https://example.org/" + string(rune('a'+i))
	}
	outcomes := fetchMetadataBatch(context.Background(), fetcher, urls)
	if len(outcomes) != len(urls) {
		t.Fatalf("outcomes = %d, want %d", len(outcomes), len(urls))
	}
	for i, outcome := range outcomes {
		if outcome.status != "ok" {
			t.Fatalf("outcome[%d] status = %q, want ok", i, outcome.status)
		}
		if outcome.meta.Title != urls[i] {
			t.Fatalf("outcome[%d] title = %q, want %q (order not deterministic)", i, outcome.meta.Title, urls[i])
		}
	}
	if max := atomic.LoadInt32(&fetcher.max); max > metadataConcurrency {
		t.Fatalf("max concurrency = %d, want <= %d", max, metadataConcurrency)
	} else if max < 2 {
		t.Fatalf("max concurrency = %d, want concurrent fetches", max)
	}
	if metadataConcurrency < 3 || metadataConcurrency > 5 {
		t.Fatalf("metadataConcurrency = %d, want 3–5", metadataConcurrency)
	}
}

func TestFetchMetadataBatchMapsErrors(t *testing.T) {
	fetcher := &fakePageFetcher{}
	outcomes := fetchMetadataBatch(context.Background(), fetcher,
		[]string{"https://example.org/ok", "https://example.org/blocked", "https://example.org/fail"})
	if outcomes[0].status != "ok" {
		t.Fatalf("ok status = %q", outcomes[0].status)
	}
	if outcomes[1].status != "blocked" {
		t.Fatalf("blocked status = %q", outcomes[1].status)
	}
	if outcomes[2].status != "failed" {
		t.Fatalf("failed status = %q", outcomes[2].status)
	}
}

func TestFetchMetadataBatchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fetcher := &fakePageFetcher{}
	outcomes := fetchMetadataBatch(ctx, fetcher, []string{"https://example.org/a", "https://example.org/b"})
	for i, outcome := range outcomes {
		if outcome.status != "not_attempted" {
			t.Fatalf("outcome[%d] status = %q, want not_attempted", i, outcome.status)
		}
	}
	if max := atomic.LoadInt32(&fetcher.max); max != 0 {
		t.Fatalf("fetcher was called %d times with a canceled context", max)
	}
}
