package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

var (
	ErrBlockedURL      = errors.New("目标地址不是公开网页")
	ErrBlockedResponse = errors.New("目标网页限制访问")
	ErrUnsupportedPage = errors.New("目标不是 HTML 页面")
)

type PageMetadata struct {
	Title        string
	Description  string
	ImageURL     string
	SiteName     string
	Author       string
	PublishedAt  *time.Time
	CanonicalURL string
	FinalURL     string
}

type MetadataFetcher struct {
	client   *http.Client
	maxBytes int64
}

func NewMetadataFetcher() *MetadataFetcher {
	dialer := &net.Dialer{
		Timeout: 8 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if !isPublicIP(ip) {
				return ErrBlockedURL
			}
			return nil
		},
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           8,
		IdleConnTimeout:        30 * time.Second,
		TLSHandshakeTimeout:    8 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("重定向超过 3 次")
			}
			_, err := ValidatePublicURL(req.URL.String())
			return err
		},
	}
	return &MetadataFetcher{client: client, maxBytes: 2 << 20}
}

// ValidatePublicURL rejects non-http(s) schemes, non-standard ports, credential
// URLs and literal internal addresses. Hostname resolution is additionally
// checked at dial time to prevent DNS rebinding.
func ValidatePublicURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: URL 无法解析", ErrBlockedURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: 仅允许 http/https", ErrBlockedURL)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: URL 不允许携带凭据", ErrBlockedURL)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("%w: 缺少主机名", ErrBlockedURL)
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return nil, fmt.Errorf("%w: 仅允许 80/443 端口", ErrBlockedURL)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, fmt.Errorf("%w: 不允许访问内网主机", ErrBlockedURL)
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && !isPublicIP(ip) {
		return nil, fmt.Errorf("%w: 不允许访问内网或保留地址", ErrBlockedURL)
	}
	return u, nil
}

func (f *MetadataFetcher) Fetch(ctx context.Context, rawURL string) (PageMetadata, error) {
	var meta PageMetadata
	u, err := ValidatePublicURL(rawURL)
	if err != nil {
		return meta, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return meta, err
	}
	req.Header.Set("User-Agent", "XiweiSpace/1.0 (metadata reader; public page summary)")
	req.Header.Set("Accept", "text/html, application/xhtml+xml;q=0.9")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.6")
	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlockedURL) {
			return meta, err
		}
		return meta, fmt.Errorf("页面读取失败: %w", err)
	}
	defer resp.Body.Close()
	meta.FinalURL = resp.Request.URL.String()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return meta, fmt.Errorf("%w: HTTP %d", ErrBlockedResponse, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return meta, fmt.Errorf("页面返回 HTTP %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return meta, ErrUnsupportedPage
	}
	body, err := readLimited(resp.Body, f.maxBytes)
	if err != nil {
		return meta, err
	}
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		reader = bytes.NewReader(body)
	}
	parsed := parseHTMLMetadata(reader, meta.FinalURL)
	meta.Title = parsed.Title
	meta.Description = parsed.Description
	meta.ImageURL = parsed.ImageURL
	meta.SiteName = parsed.SiteName
	meta.Author = parsed.Author
	meta.CanonicalURL = parsed.CanonicalURL
	meta.PublishedAt = validPublicationTime(parsed.PublishedAt)
	if looksLikeInterstitial(meta.FinalURL, parsed.Title, parsed.TextSample) {
		return meta, fmt.Errorf("%w: 页面要求登录、验证码或安全验证", ErrBlockedResponse)
	}
	return meta, nil
}

type parsedPage struct {
	Title        string
	Description  string
	ImageURL     string
	SiteName     string
	Author       string
	CanonicalURL string
	PublishedAt  *time.Time
	TextSample   string
}

func parseHTMLMetadata(r io.Reader, finalURL string) parsedPage {
	var out parsedPage
	meta := map[string]string{}
	var inTitle, inScript bool
	var title, script strings.Builder
	var text strings.Builder
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			tag := strings.ToLower(string(name))
			attrs := map[string]string{}
			if hasAttr {
				for {
					key, val, more := z.TagAttr()
					attrs[strings.ToLower(string(key))] = string(val)
					if !more {
						break
					}
				}
			}
			switch tag {
			case "title":
				inTitle = true
			case "script":
				inScript = true
				script.Reset()
			case "meta":
				key := firstNonEmpty(attrs["property"], attrs["name"], attrs["itemprop"])
				key = strings.ToLower(strings.TrimSpace(key))
				if key != "" && attrs["content"] != "" {
					meta[key] = strings.TrimSpace(html.UnescapeString(attrs["content"]))
				}
			case "link":
				if strings.EqualFold(attrs["rel"], "canonical") && attrs["href"] != "" {
					out.CanonicalURL = strings.TrimSpace(attrs["href"])
				}
			case "time":
				if out.PublishedAt == nil && attrs["datetime"] != "" {
					out.PublishedAt = parseMetadataTime(attrs["datetime"])
				}
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch strings.ToLower(string(name)) {
			case "title":
				inTitle = false
			case "script":
				inScript = false
				if script.Len() > 0 {
					applyJSONLD(&out, script.String())
				}
			}
		case html.TextToken:
			textValue := string(z.Text())
			if inTitle {
				title.WriteString(textValue)
			}
			if inScript {
				if script.Len() < 256<<10 {
					script.WriteString(textValue)
				}
				continue
			}
			if text.Len() < 64<<10 {
				text.WriteString(textValue)
				text.WriteByte(' ')
			}
		}
	}
	out.TextSample = collapseSpace(text.String())
	out.Title = firstNonEmpty(meta["og:title"], meta["twitter:title"], meta["headline"], collapseSpace(title.String()))
	out.Description = firstNonEmpty(meta["og:description"], meta["twitter:description"], meta["description"])
	out.ImageURL = firstNonEmpty(meta["og:image"], meta["twitter:image"], meta["twitter:image:src"])
	out.SiteName = firstNonEmpty(meta["og:site_name"], meta["application-name"])
	out.Author = firstNonEmpty(meta["author"], meta["article:author"])
	if out.PublishedAt == nil {
		out.PublishedAt = parseMetadataTime(firstNonEmpty(
			meta["article:published_time"], meta["datepublished"], meta["pubdate"],
			meta["publishdate"], meta["date"], meta["dc.date"],
		))
	}
	out.Title = truncateText(out.Title, 300)
	out.Description = truncateText(out.Description, 1200)
	out.Author = truncateText(out.Author, 120)
	if out.CanonicalURL != "" {
		if absolute, err := resolveURL(finalURL, out.CanonicalURL); err == nil {
			out.CanonicalURL = absolute
		}
	}
	if out.ImageURL != "" {
		if absolute, err := resolveURL(finalURL, out.ImageURL); err == nil {
			out.ImageURL = absolute
		}
	}
	return out
}

func applyJSONLD(out *parsedPage, raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return
	}
	walkJSONLD(v, out, 0)
}

func walkJSONLD(v any, out *parsedPage, depth int) {
	if depth > 4 {
		return
	}
	switch value := v.(type) {
	case []any:
		for _, item := range value {
			walkJSONLD(item, out, depth+1)
		}
	case map[string]any:
		if out.PublishedAt == nil {
			if s, ok := value["datePublished"].(string); ok {
				out.PublishedAt = parseMetadataTime(s)
			}
		}
		if out.Title == "" {
			if s, ok := value["headline"].(string); ok {
				out.Title = collapseSpace(s)
			}
		}
		if out.Author == "" {
			switch author := value["author"].(type) {
			case string:
				out.Author = collapseSpace(author)
			case map[string]any:
				if s, ok := author["name"].(string); ok {
					out.Author = collapseSpace(s)
				}
			case []any:
				for _, item := range author {
					if m, ok := item.(map[string]any); ok {
						if s, ok := m["name"].(string); ok && s != "" {
							out.Author = collapseSpace(s)
							break
						}
					}
				}
			}
		}
		for _, key := range []string{"@graph", "mainEntity", "itemListElement"} {
			if nested, ok := value[key]; ok {
				walkJSONLD(nested, out, depth+1)
			}
		}
	}
}

func parseMetadataTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if t := parseBingDate(raw); t != nil {
		return t
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02",
		"2006年01月02日 15:04",
		"2006年01月02日",
	} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

func validPublicationTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	if t.Year() < 1990 || t.After(time.Now().Add(48*time.Hour)) {
		return nil
	}
	return t
}

func looksLikeInterstitial(finalURL, title, text string) bool {
	lowerURL := strings.ToLower(finalURL)
	lowerTitle := strings.ToLower(title)
	for _, marker := range []string{"/login", "/signin", "/passport", "captcha", "/verify", "/security-check"} {
		if strings.Contains(lowerURL, marker) {
			return true
		}
	}
	for _, marker := range []string{"验证码", "安全验证", "请登录", "登录后查看", "访问受限", "captcha", "access denied", "sign in to continue"} {
		if strings.Contains(lowerTitle, marker) {
			return true
		}
	}
	sample := strings.ToLower(text[:min(len(text), 4096)])
	for _, marker := range []string{"请输入验证码", "完成安全验证", "请登录后", "captcha", "access denied"} {
		if strings.Contains(sample, marker) {
			return true
		}
	}
	return false
}

func resolveURL(base, ref string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(refURL).String(), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateText(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}
