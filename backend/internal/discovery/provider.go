package discovery

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"xiweispace/backend/internal/models"
)

// SearchRequest is provider-neutral. Providers may ignore unsupported fields,
// but must not silently broaden a domain or time constraint when they do apply it.
type SearchRequest struct {
	Query    string
	Domains  []string
	Since    *time.Time
	Until    *time.Time
	Page     int
	PageSize int
}

type SearchResult struct {
	Title       string
	URL         string
	Snippet     string
	PublishedAt *time.Time
	SourceName  string
	Platform    string
	Provider    string
}

type SearchProvider interface {
	Name() string
	Validate() error
	Search(ctx context.Context, req SearchRequest) ([]SearchResult, error)
}

// NewSearchProvider builds a provider from stored configuration. API keys are
// resolved from the environment by variable name and are never stored in the DB.
func NewSearchProvider(p models.SearchProvider) (SearchProvider, error) {
	apiKey := ""
	if p.APIKeyEnv != "" {
		apiKey = os.Getenv(p.APIKeyEnv)
	}
	switch p.ProviderType {
	case "bing_rss":
		return NewBingRSSProvider(p.Endpoint, p.Config), nil
	case "searxng":
		return NewSearxNGProvider(p.Endpoint, apiKey, p.Config), nil
	default:
		return nil, fmt.Errorf("不支持的搜索服务类型 %q", p.ProviderType)
	}
}

func cleanPage(req SearchRequest) (int, int) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.PageSize
	if size < 1 {
		size = 10
	}
	if size > 50 {
		size = 50
	}
	return page, size
}

func domainClause(domains []string) string {
	parts := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://"))
		d = strings.TrimSuffix(strings.SplitN(d, "/", 2)[0], ".")
		if d != "" {
			parts = append(parts, "site:"+d)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// SourceIdentity is the publisher identity derived from a result URL. Known is
// false for hosts that are not in the configured platform list; callers must
// keep those hosts conservative (media) instead of promoting them.
type SourceIdentity struct {
	Name       string
	Platform   string
	SourceType string
	Known      bool
}

// IdentifySource maps a publisher host to a human-readable platform and a
// conservative source type. Matching is on hostname boundaries only: the host
// must equal the root domain or be a subdomain of it. This prevents
// "fakeweibo.com" and "weibo.com.attacker.tld" from being treated as Weibo.
func IdentifySource(rawURL string) SourceIdentity {
	host := normalizeHost(hostOf(rawURL))
	if host == "" {
		return SourceIdentity{SourceType: "media"}
	}
	name := strings.TrimPrefix(host, "www.")
	identity := SourceIdentity{Name: name, Platform: name, SourceType: "media"}

	switch {
	case hostMatches(host, "weibo.com") || hostMatches(host, "weibo.cn"):
		identity = sourceIdentity("微博", "微博", "community")
	case hostMatches(host, "xiaohongshu.com"):
		identity = sourceIdentity("小红书", "小红书", "community")
	case hostMatches(host, "douyin.com"):
		identity = sourceIdentity("抖音", "抖音", "community")
	case hostMatches(host, "bilibili.com"):
		identity = sourceIdentity("哔哩哔哩", "哔哩哔哩", "community")
	case hostMatches(host, "zhihu.com"):
		identity = sourceIdentity("知乎", "知乎", "community")
	case hostMatches(host, "baike.baidu.com") || hostMatches(host, "wikipedia.org"):
		identity = sourceIdentity("百科资料", "百科", "reference")
	case hostMatches(host, "weixin.qq.com"):
		identity = sourceIdentity("微信公众号", "微信", "media")
	case hostMatches(host, "sohu.com"):
		identity = sourceIdentity("搜狐", "搜狐", "media")
	case hostMatches(host, "sina.com.cn") || hostMatches(host, "sina.cn"):
		identity = sourceIdentity("新浪", "新浪", "media")
	case hostMatches(host, "163.com"):
		identity = sourceIdentity("网易", "网易", "media")
	case hostMatches(host, "qq.com"):
		identity = sourceIdentity("腾讯新闻", "腾讯", "media")
	case hostMatches(host, "ifeng.com"):
		identity = sourceIdentity("凤凰网", "凤凰网", "media")
	case hostMatches(host, "thepaper.cn"):
		identity = sourceIdentity("澎湃新闻", "澎湃", "media")
	case hostMatches(host, "cctv.com") || hostMatches(host, "cnr.cn") ||
		hostMatches(host, "xinhuanet.com") || hostMatches(host, "people.com.cn"):
		identity = sourceIdentity("官方媒体", "官方媒体", "official")
	case hostMatches(host, "baidu.com"):
		identity = sourceIdentity("百度", "百度", "reference")
	}
	return identity
}

func sourceIdentity(name, platform, sourceType string) SourceIdentity {
	return SourceIdentity{Name: name, Platform: platform, SourceType: sourceType, Known: true}
}

// PlatformForURL is the legacy three-value form of IdentifySource.
func PlatformForURL(rawURL string) (name, platform, sourceType string) {
	identity := IdentifySource(rawURL)
	return identity.Name, identity.Platform, identity.SourceType
}

// hostMatches reports whether host is the root domain itself or a subdomain.
// Ports are removed by url.Hostname; case and trailing dots are normalized.
func hostMatches(host, root string) bool {
	host = normalizeHost(host)
	root = normalizeHost(root)
	if host == "" || root == "" {
		return false
	}
	return host == root || strings.HasSuffix(host, "."+root)
}

func normalizeHost(host string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(host)), ".")
}
