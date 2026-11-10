package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	Role      string    `json:"role"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthSession struct {
	User      User
	CSRFToken string
	ExpiresAt time.Time
}

type Artist struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	NameEN      string         `json:"name_en"`
	Avatar      string         `json:"avatar"`
	Cover       string         `json:"cover"`
	Bio         string         `json:"bio"`
	ProfileData map[string]any `json:"profile_data"`
}

type Scene struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sort_order"`
	Enabled     bool   `json:"enabled"`
	Weight      int    `json:"weight,omitempty"`
	Selected    bool   `json:"selected,omitempty"`
	IsDefault   bool   `json:"is_default,omitempty"`
}

type WorkRef struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	ReleaseDate string `json:"release_date"`
	Cover       string `json:"cover"`
}

type CharacterRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	WorkID    string `json:"work_id"`
	WorkTitle string `json:"work_title,omitempty"`
}

type Work struct {
	ID          string      `json:"id"`
	ArtistID    string      `json:"artist_id"`
	Title       string      `json:"title"`
	Type        string      `json:"type"`
	Cover       string      `json:"cover"`
	Description string      `json:"description"`
	ReleaseDate string      `json:"release_date"`
	Status      string      `json:"status"`
	Platform    string      `json:"platform"`
	OfficialURL string      `json:"official_url"`
	Characters  []Character `json:"characters,omitempty"`
	Favorited   bool        `json:"favorited,omitempty"`
}

type Character struct {
	ID          string   `json:"id"`
	WorkID      string   `json:"work_id"`
	Name        string   `json:"name"`
	Avatar      string   `json:"avatar"`
	Description string   `json:"description"`
	Work        *WorkRef `json:"work,omitempty"`
	Favorited   bool     `json:"favorited,omitempty"`
}

type Content struct {
	ID                 string          `json:"id"`
	Title              string          `json:"title"`
	Summary            string          `json:"summary"`
	ContentType        string          `json:"content_type"`
	SourceID           *string         `json:"source_id,omitempty"`
	SourceName         string          `json:"source_name"`
	SourcePlatform     string          `json:"source_platform"`
	SourceType         string          `json:"source_type"`
	SourceAuthor       string          `json:"source_author"`
	SourceURL          string          `json:"source_url"`
	CoverURL           string          `json:"cover_url"`
	PublishedAt        *time.Time      `json:"published_at"`
	EffectiveAt        time.Time       `json:"effective_at"`
	DiscoveredAt       time.Time       `json:"discovered_at"`
	PublishedPrecision string          `json:"published_precision"`
	CollectedAt        time.Time       `json:"collected_at"`
	CreatedAt          time.Time       `json:"created_at"`
	OriginName         string          `json:"origin_name"`
	OriginPlatform     string          `json:"origin_platform"`
	OriginType         string          `json:"origin_type"`
	MetadataStatus     string          `json:"metadata_status"`
	MetadataError      string          `json:"metadata_error,omitempty"`
	SearchTaskID       *string         `json:"search_task_id,omitempty"`
	TrustLevel         string          `json:"trust_level"`
	IsOfficial         bool            `json:"is_official"`
	ReviewStatus       string          `json:"review_status"`
	ContentStatus      string          `json:"content_status"`
	ReviewReason       string          `json:"review_reason,omitempty"`
	Scenes             []Scene         `json:"scenes"`
	Works              []WorkRef       `json:"works"`
	Characters         []CharacterRef  `json:"characters"`
	Events             []EventRef      `json:"events"`
	Favorited          bool            `json:"favorited"`
	Unavailable        bool            `json:"unavailable,omitempty"`
	RawData            json.RawMessage `json:"raw_data,omitempty"`
}

type EventRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"event_date"`
}

type Event struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	CoverURL     string    `json:"cover_url"`
	EventDate    string    `json:"event_date"`
	Location     string    `json:"location"`
	Status       string    `json:"status"`
	ContentCount int       `json:"content_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type EventDetail struct {
	Event    Event     `json:"event"`
	Contents []Content `json:"contents"`
}

type SearchProvider struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	ProviderType  string         `json:"provider_type"`
	Endpoint      string         `json:"endpoint"`
	APIKeyEnv     string         `json:"api_key_env"`
	Config        map[string]any `json:"config"`
	Enabled       bool           `json:"enabled"`
	LastTestAt    *time.Time     `json:"last_test_at"`
	LastTestOK    *bool          `json:"last_test_ok"`
	LastTestError string         `json:"last_test_error"`
	LastLatencyMS int            `json:"last_latency_ms"`
	CreatedAt     time.Time      `json:"created_at"`
}

type SearchTask struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	ProviderID      string     `json:"provider_id"`
	ProviderName    string     `json:"provider_name"`
	Query           string     `json:"query"`
	Domains         []string   `json:"domains"`
	ContentTypeHint string     `json:"content_type_hint"`
	SourceType      string     `json:"source_type"`
	TrustLevel      string     `json:"trust_level"`
	IntervalMinutes int        `json:"interval_minutes"`
	MaxResults      int        `json:"max_results"`
	Enabled         bool       `json:"enabled"`
	LastRunAt       *time.Time `json:"last_run_at"`
	LastSuccessAt   *time.Time `json:"last_success_at"`
	LastFailureAt   *time.Time `json:"last_failure_at"`
	LastError       string     `json:"last_error"`
	NextRunAt       time.Time  `json:"next_run_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type SearchRun struct {
	ID               string     `json:"id"`
	TaskID           string     `json:"task_id"`
	TaskName         string     `json:"task_name"`
	Status           string     `json:"status"`
	TriggerType      string     `json:"trigger_type"`
	StartedAt        *time.Time `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	FetchedCount     int        `json:"fetched_count"`
	InsertedCount    int        `json:"inserted_count"`
	DuplicateCount   int        `json:"duplicate_count"`
	FailedCount      int        `json:"failed_count"`
	ParsedOKCount    int        `json:"parsed_ok_count"`
	ParseFailedCount int        `json:"parse_failed_count"`
	ErrorMessage     string     `json:"error_message"`
	CreatedAt        time.Time  `json:"created_at"`
}

type SearchTestResult struct {
	OK          bool   `json:"ok"`
	LatencyMS   int64  `json:"latency_ms"`
	ResultCount int    `json:"result_count"`
	Message     string `json:"message"`
}

type FavoriteItem struct {
	ID          string         `json:"id"`
	TargetType  string         `json:"target_type"`
	TargetID    string         `json:"target_id"`
	Title       string         `json:"title"`
	Subtitle    string         `json:"subtitle"`
	SourceURL   string         `json:"source_url"`
	Cover       string         `json:"cover"`
	CreatedAt   time.Time      `json:"created_at"`
	Unavailable bool           `json:"unavailable"`
	Snapshot    map[string]any `json:"snapshot,omitempty"`
}

type Source struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Platform        string         `json:"platform"`
	SourceType      string         `json:"source_type"`
	AdapterType     string         `json:"adapter_type"`
	Config          map[string]any `json:"config"`
	TrustLevel      string         `json:"trust_level"`
	Enabled         bool           `json:"enabled"`
	IntervalMinutes int            `json:"interval_minutes"`
	LastSuccessAt   *time.Time     `json:"last_success_at"`
	LastFailureAt   *time.Time     `json:"last_failure_at"`
	LastError       string         `json:"last_error"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Job struct {
	ID             string     `json:"id"`
	SourceID       string     `json:"source_id"`
	SourceName     string     `json:"source_name"`
	Status         string     `json:"status"`
	TriggerType    string     `json:"trigger_type"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	FetchedCount   int        `json:"fetched_count"`
	InsertedCount  int        `json:"inserted_count"`
	DuplicateCount int        `json:"duplicate_count"`
	FailedCount    int        `json:"failed_count"`
	ErrorMessage   string     `json:"error_message"`
	CreatedAt      time.Time  `json:"created_at"`
}

type DailyBrief struct {
	Date        string     `json:"date"`
	NewCount    int        `json:"new_count"`
	LastUpdated *time.Time `json:"last_updated_at"`
}

type HomeData struct {
	Artist       Artist     `json:"artist"`
	Scenes       []Scene    `json:"scenes"`
	DefaultScene string     `json:"default_scene"`
	Latest       []Content  `json:"latest"`
	ForYou       []Content  `json:"for_you"`
	Featured     []Content  `json:"featured"`
	Works        []Work     `json:"works"`
	Brief        DailyBrief `json:"brief"`
}

type CharacterDetail struct {
	Character Character `json:"character"`
	Contents  []Content `json:"contents"`
}

type WorkDetail struct {
	Work     Work      `json:"work"`
	Contents []Content `json:"contents"`
}

type Paged[T any] struct {
	Items      []T    `json:"items"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	Total      int    `json:"total"`
	NextCursor string `json:"next_cursor,omitempty"`
}
