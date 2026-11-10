package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SearxNGProvider struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

type searxResponse struct {
	Results []struct {
		Title         string `json:"title"`
		URL           string `json:"url"`
		Content       string `json:"content"`
		PublishedDate string `json:"publishedDate"`
		Engine        string `json:"engine"`
	} `json:"results"`
}

func NewSearxNGProvider(endpoint, apiKey string, _ map[string]any) *SearxNGProvider {
	return &SearxNGProvider{
		endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 18 * time.Second},
	}
}

func (p *SearxNGProvider) Name() string { return "SearXNG" }

func (p *SearxNGProvider) Validate() error {
	u, err := url.Parse(p.endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("SearXNG 地址必须是有效的 http(s) URL")
	}
	return nil
}

func (p *SearxNGProvider) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	if err := p.Validate(); err != nil {
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
	u, err := url.Parse(p.endpoint + "/search")
	if err != nil {
		return nil, err
	}
	values := u.Query()
	values.Set("q", query)
	values.Set("format", "json")
	values.Set("pageno", strconv.Itoa(page))
	values.Set("language", "zh-CN")
	u.RawQuery = values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "XiweiSpace/1.0 (content discovery)")
	httpReq.Header.Set("Accept", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(httpReq)
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
	var parsed searxResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("搜索响应不是有效 JSON: %w", err)
	}
	out := make([]SearchResult, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		link := strings.TrimSpace(item.URL)
		if link == "" || hostOf(link) == "" {
			continue
		}
		sourceName, platform, _ := PlatformForURL(link)
		published := parseBingDate(item.PublishedDate)
		if !withinWindow(published, req.Since, req.Until) {
			continue
		}
		out = append(out, SearchResult{
			Title:       strings.TrimSpace(item.Title),
			URL:         link,
			Snippet:     strings.TrimSpace(item.Content),
			PublishedAt: published,
			SourceName:  sourceName,
			Platform:    platform,
			Provider:    p.Name(),
		})
		if len(out) >= size {
			break
		}
	}
	return out, nil
}
