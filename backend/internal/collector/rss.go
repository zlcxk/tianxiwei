package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"xiweispace/backend/internal/models"
)

type RSSAdapter struct {
	name          string
	feedURL       string
	keywords      []string
	useFeedImages bool
	client        *http.Client
}

func NewRSSAdapter(source models.Source) (*RSSAdapter, error) {
	feedURL, _ := source.Config["feed_url"].(string)
	feedURL = strings.TrimSpace(feedURL)
	if feedURL == "" {
		return nil, fmt.Errorf("rss config must include feed_url")
	}
	u, err := url.Parse(feedURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("feed_url must be an http(s) URL")
	}
	keywords := stringSlice(source.Config["keywords"])
	useFeedImages, _ := source.Config["use_feed_images"].(bool)
	return &RSSAdapter{
		name:          source.Name,
		feedURL:       feedURL,
		keywords:      keywords,
		useFeedImages: useFeedImages,
		client:        &http.Client{Timeout: 25 * time.Second},
	}, nil
}

func (a *RSSAdapter) Name() string { return a.name }

func (a *RSSAdapter) Validate() error {
	if a.feedURL == "" {
		return fmt.Errorf("feed_url is empty")
	}
	return nil
}

func (a *RSSAdapter) Fetch(ctx context.Context, _ string) ([]RawItem, string, error) {
	parser := gofeed.NewParser()
	parser.Client = a.client
	parser.UserAgent = "XiweiSpaceBot/1.0 (+public RSS reader; contact: site administrator)"

	feed, err := parser.ParseURLWithContext(a.feedURL, ctx)
	if err != nil {
		return nil, "", fmt.Errorf("fetch RSS: %w", err)
	}
	out := make([]RawItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		text := item.Title + "\n" + item.Description + "\n" + item.Content
		if !a.matches(text) {
			continue
		}
		var published *time.Time
		if item.PublishedParsed != nil {
			t := *item.PublishedParsed
			published = &t
		} else if item.UpdatedParsed != nil {
			t := *item.UpdatedParsed
			published = &t
		}
		author := ""
		if item.Author != nil {
			author = item.Author.Name
		}
		// 第三方图片默认不落地展示；只有管理员在来源配置中显式授权后才引用。
		cover := ""
		if a.useFeedImages {
			if item.Image != nil {
				cover = item.Image.URL
			}
			for _, enc := range item.Enclosures {
				if cover == "" && strings.HasPrefix(enc.Type, "image/") {
					cover = enc.URL
				}
			}
		}
		raw, _ := json.Marshal(map[string]any{
			"guid":        item.GUID,
			"link":        item.Link,
			"title":       item.Title,
			"description": truncateRunes(item.Description, 2000),
			"published":   item.Published,
			"author":      author,
		})
		itemID := strings.TrimSpace(item.GUID)
		if itemID == "" {
			itemID = strings.TrimSpace(item.Link)
		}
		out = append(out, RawItem{
			SourceItemID: itemID,
			Title:        strings.TrimSpace(item.Title),
			Summary:      truncateRunes(stripHTML(item.Description+" "+item.Content), 600),
			URL:          strings.TrimSpace(item.Link),
			CoverURL:     cover,
			Author:       author,
			PublishedAt:  published,
			Raw:          raw,
		})
		if len(out) >= 100 {
			break
		}
	}
	return out, "", nil
}

func (a *RSSAdapter) matches(text string) bool {
	if len(a.keywords) == 0 {
		return true
	}
	lower := strings.ToLower(text)
	for _, kw := range a.keywords {
		if strings.Contains(lower, strings.ToLower(strings.TrimSpace(kw))) {
			return true
		}
	}
	return false
}

func stringSlice(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return t
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{strings.TrimSpace(t)}
	default:
		return nil
	}
}
