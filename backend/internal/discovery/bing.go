package discovery

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const bingDefaultEndpoint = "https://www.bing.com/search"

type BingRSSProvider struct {
	endpoint string
	locale   string
	client   *http.Client
}

type bingRSSDoc struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

func NewBingRSSProvider(endpoint string, cfg map[string]any) *BingRSSProvider {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = bingDefaultEndpoint
	}
	locale := ""
	if cfg != nil {
		if v, ok := cfg["locale"].(string); ok {
			locale = v
		}
	}
	return &BingRSSProvider{
		endpoint: endpoint,
		locale:   locale,
		client:   &http.Client{Timeout: 18 * time.Second},
	}
}

func (b *BingRSSProvider) Name() string { return "Bing Web Search RSS" }

func (b *BingRSSProvider) Validate() error {
	u, err := url.Parse(b.endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("搜索服务地址必须是公开的 http(s) URL")
	}
	return nil
}

func (b *BingRSSProvider) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	page, size := cleanPage(req)
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	if clause := domainClause(req.Domains); clause != "" {
		query += " " + clause
	}
	u, _ := url.Parse(b.endpoint)
	values := u.Query()
	values.Set("q", query)
	values.Set("format", "rss")
	values.Set("count", strconv.Itoa(size))
	values.Set("first", strconv.Itoa((page-1)*size+1))
	if b.locale != "" {
		values.Set("setlang", b.locale)
	}
	u.RawQuery = values.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "XiweiSpace/1.0 (content discovery; public RSS aggregation)")
	httpReq.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, text/xml;q=0.8")
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("搜索请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("搜索服务返回 HTTP %d", resp.StatusCode)
	}
	body, err := readLimited(resp.Body, 4<<20)
	if err != nil {
		return nil, err
	}
	var doc bingRSSDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("搜索响应不是有效的 RSS: %w", err)
	}
	out := make([]SearchResult, 0, len(doc.Channel.Items))
	for _, item := range doc.Channel.Items {
		link := strings.TrimSpace(item.Link)
		if link == "" || hostOf(link) == "" {
			continue
		}
		sourceName, platform, _ := PlatformForURL(link)
		published := parseBingDate(item.PubDate)
		if !withinWindow(published, req.Since, req.Until) {
			continue
		}
		out = append(out, SearchResult{
			Title:       strings.TrimSpace(item.Title),
			URL:         link,
			Snippet:     strings.TrimSpace(item.Description),
			PublishedAt: published,
			SourceName:  sourceName,
			Platform:    platform,
			Provider:    b.Name(),
		})
	}
	return out, nil
}
