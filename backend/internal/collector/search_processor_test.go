package collector

import (
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
