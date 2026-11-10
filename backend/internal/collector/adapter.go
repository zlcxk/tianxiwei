package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"xiweispace/backend/internal/models"
)

type RawItem struct {
	SourceItemID string          `json:"source_item_id"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	URL          string          `json:"url"`
	CoverURL     string          `json:"cover_url"`
	Author       string          `json:"author"`
	PublishedAt  time.Time       `json:"published_at"`
	Raw          json.RawMessage `json:"raw"`
}

type SourceAdapter interface {
	Name() string
	Validate() error
	Fetch(ctx context.Context, cursor string) ([]RawItem, string, error)
}

func NewAdapter(source models.Source) (SourceAdapter, error) {
	switch source.AdapterType {
	case "rss":
		return NewRSSAdapter(source)
	case "manual":
		return nil, fmt.Errorf("manual sources do not have a fetch adapter")
	default:
		return nil, fmt.Errorf("unsupported adapter type %q", source.AdapterType)
	}
}
