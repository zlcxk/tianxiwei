package collector

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

func stripHTML(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	z := html.NewTokenizer(strings.NewReader(input))
	var b strings.Builder
	skip := 0
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return strings.Join(strings.Fields(b.String()), " ")
		case html.StartTagToken:
			name, _ := z.TagName()
			if string(name) == "script" || string(name) == "style" {
				skip++
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			if (string(name) == "script" || string(name) == "style") && skip > 0 {
				skip--
			}
		case html.TextToken:
			if skip == 0 {
				b.Write(z.Text())
				b.WriteByte(' ')
			}
		}
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func canonicalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	q := u.Query()
	for key := range q {
		lk := strings.ToLower(key)
		if strings.HasPrefix(lk, "utm_") || lk == "spm" || lk == "from" || lk == "share_token" || lk == "scene" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	if u.Path != "/" {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String()
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

func contentFingerprint(title, summary string) string {
	return hashString(strings.Join(strings.Fields(title+"\n"+summary), " "))
}

func splitKeywords(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；' || r == ' ' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if v := strings.TrimSpace(f); v != "" {
			out = append(out, v)
		}
	}
	return out
}

type Classification struct {
	SceneIDs []string
	Scenes   []string
}

func Classify(text string, rules []store.Rule) Classification {
	lower := strings.ToLower(text)
	seen := map[string]bool{}
	var out Classification
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		for _, kw := range splitKeywords(rule.Keywords) {
			if kw == "" {
				continue
			}
			if strings.Contains(lower, strings.ToLower(kw)) {
				if !seen[rule.SceneID] {
					seen[rule.SceneID] = true
					out.SceneIDs = append(out.SceneIDs, rule.SceneID)
					out.Scenes = append(out.Scenes, rule.SceneName)
				}
				break
			}
		}
	}
	sort.Strings(out.Scenes)
	return out
}

var importantKeywords = []string{
	"官宣", "定档", "开机", "杀青", "获奖", "荣誉", "恋情", "声明", "律师函",
	"代言", "发布会", "封面", "争议", "道歉", "解约", "签约", "结婚", "婚讯",
	"去世", "讣告", "番位", "票房", "收视", "起诉", "风波",
}

type ReviewDecision struct {
	ReviewStatus  string
	ContentStatus string
	Reason        string
}

func DecideReview(source models.Source, text string, sceneCount int) ReviewDecision {
	lower := strings.ToLower(text)
	for _, kw := range importantKeywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return ReviewDecision{
				ReviewStatus:  "pending",
				ContentStatus: "reviewing",
				Reason:        "涉及重要信息「" + kw + "」，发布前需人工核验",
			}
		}
	}
	if source.SourceType == "community" || source.TrustLevel == "community" {
		return ReviewDecision{
			ReviewStatus:  "pending",
			ContentStatus: "reviewing",
			Reason:        "来源为社区内容，需人工核验",
		}
	}
	if sceneCount == 0 {
		return ReviewDecision{
			ReviewStatus:  "pending",
			ContentStatus: "reviewing",
			Reason:        "内容分类不确定，需人工归类",
		}
	}
	return ReviewDecision{ReviewStatus: "none", ContentStatus: "published"}
}

// inferContentTypeFromText looks for concrete content-type signals in the
// actual title, summary or page metadata. The boolean reports whether a
// positive signal was found; source-type defaults are handled by the caller.
func inferContentTypeFromText(text string) (string, bool) {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "采访") || strings.Contains(lower, "专访") || strings.Contains(lower, "访谈"):
		return "interview", true
	case strings.Contains(lower, "视频") || strings.Contains(lower, "直播") || strings.Contains(lower, "vlog"):
		return "video", true
	case strings.Contains(lower, "综艺") && strings.Contains(lower, "节目"):
		return "variety", true
	case strings.Contains(lower, "剧集") || strings.Contains(lower, "角色") || strings.Contains(lower, "开播") ||
		strings.Contains(lower, "电视剧") || strings.Contains(lower, "开机") || strings.Contains(lower, "杀青") ||
		strings.Contains(lower, "定档") || strings.Contains(lower, "剧照"):
		return "work", true
	case strings.Contains(lower, "品牌") || strings.Contains(lower, "代言") || strings.Contains(lower, "商务"):
		return "fashion", true
	case strings.Contains(lower, "写真") || strings.Contains(lower, "杂志") || strings.Contains(lower, "造型") || strings.Contains(lower, "图集"):
		return "image", true
	case strings.Contains(lower, "发布会") || strings.Contains(lower, "见面会") || strings.Contains(lower, "红毯") ||
		strings.Contains(lower, "活动") || strings.Contains(lower, "出席"):
		return "event", true
	case strings.Contains(lower, "官方公告") || strings.Contains(lower, "声明") || strings.Contains(lower, "公告"):
		return "official", true
	}
	return "", false
}

func InferContentType(source models.Source, text string) string {
	if contentType, ok := inferContentTypeFromText(text); ok {
		return contentType
	}
	switch source.SourceType {
	case "official":
		return "social"
	case "media":
		return "media"
	default:
		return "other"
	}
}
