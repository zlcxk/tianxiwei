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

// PlatformForURL maps a publisher host to a human-readable platform and a
// conservative source type. Unknown hosts stay "media" rather than being
// promoted to official.
func PlatformForURL(rawURL string) (name, platform, sourceType string) {
	host := hostOf(rawURL)
	name = host
	if name == "" {
		return "", "", "media"
	}
	name = strings.TrimPrefix(name, "www.")
	switch {
	case strings.HasSuffix(host, "weibo.com") || strings.HasSuffix(host, "weibo.cn"):
		return "微博", "微博", "community"
	case strings.HasSuffix(host, "xiaohongshu.com"):
		return "小红书", "小红书", "community"
	case strings.HasSuffix(host, "douyin.com"):
		return "抖音", "抖音", "community"
	case strings.HasSuffix(host, "bilibili.com"):
		return "哔哩哔哩", "哔哩哔哩", "community"
	case strings.HasSuffix(host, "zhihu.com"):
		return "知乎", "知乎", "community"
	case strings.HasSuffix(host, "baike.baidu.com") || strings.HasSuffix(host, "wikipedia.org"):
		return "百科资料", "百科", "reference"
	case strings.HasSuffix(host, "weixin.qq.com") || strings.HasSuffix(host, "mp.weixin.qq.com"):
		return "微信公众号", "微信", "media"
	case strings.HasSuffix(host, "sohu.com"):
		return "搜狐", "搜狐", "media"
	case strings.HasSuffix(host, "sina.com.cn") || strings.HasSuffix(host, "sina.cn"):
		return "新浪", "新浪", "media"
	case strings.HasSuffix(host, "163.com"):
		return "网易", "网易", "media"
	case strings.HasSuffix(host, "qq.com"):
		return "腾讯新闻", "腾讯", "media"
	case strings.HasSuffix(host, "ifeng.com"):
		return "凤凰网", "凤凰网", "media"
	case strings.HasSuffix(host, "thepaper.cn"):
		return "澎湃新闻", "澎湃", "media"
	case strings.HasSuffix(host, "cctv.com") || strings.HasSuffix(host, "cnr.cn") || strings.HasSuffix(host, "xinhuanet.com") || strings.HasSuffix(host, "people.com.cn"):
		return "官方媒体", "官方媒体", "official"
	case strings.HasSuffix(host, "baidu.com"):
		return "百度", "百度", "reference"
	default:
		return name, name, "media"
	}
}
