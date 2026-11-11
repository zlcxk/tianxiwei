package collector

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

// Relevance thresholds from the product spec: >=70 is high-confidence content,
// 40-69 stays in the candidate pool for review, below 40 is filtered out.
const (
	ScoreTitle           = 40
	ScoreBody            = 25
	ScoreVerifiedAccount = 25
	ScoreRelated         = 15

	thresholdHigh      = 70
	thresholdCandidate = 40

	maxRelatedSignals = 2
	lowContextPenalty = 20
)

// Relevance labels stored on contents.relevance_label.
const (
	LabelHigh      = "high"
	LabelCandidate = "candidate"
	LabelFiltered  = "filtered"
)

// Filter reasons stored on contents.filter_reason. They are a closed set that
// matches the CHECK constraint in migration 0006.
const (
	FilterIrrelevantPerson  = "irrelevant_person"
	FilterAmbiguousEntity   = "ambiguous_entity"
	FilterDictionaryContent = "dictionary_content"
	FilterUnrelatedPerson   = "unrelated_person"
	FilterInsufficientCtx   = "insufficient_context"
	FilterDuplicateContent  = "duplicate_content"
	FilterInvalidSource     = "invalid_source"
)

// dictionaryMarkers detect dictionary / surname encyclopedia pages that share
// the artist's surname but are not about the artist. They only apply when the
// full artist name is absent from the text.
var dictionaryMarkers = []string{
	"拼音", "部首", "笔画", "笔顺", "组词", "造句", "田字格",
	"百家姓", "姓氏", "字义", "释义", "汉字",
}

// Entity is the subset of store.EntityContext needed to score a search result.
type Entity struct {
	Name             string
	Aliases          []string
	VerifiedAccounts []string
	ExcludedTerms    []string
	WorkTitles       []string
	CharacterNames   []string
	EventTitles      []string
}

// EntityFromContext adapts the store representation for scoring.
func EntityFromContext(ctx store.EntityContext) Entity {
	return Entity{
		Name:             ctx.Entity.Name,
		Aliases:          ctx.Entity.Aliases,
		VerifiedAccounts: ctx.Entity.VerifiedAccounts,
		ExcludedTerms:    ctx.Entity.ExcludedTerms,
		WorkTitles:       ctx.WorkTitles,
		CharacterNames:   ctx.CharacterNames,
		EventTitles:      ctx.EventTitles,
	}
}

// EntityFromModel builds an Entity from just the editorial config.
func EntityFromModel(entity models.ArtistEntity) Entity {
	return Entity{
		Name:             entity.Name,
		Aliases:          entity.Aliases,
		VerifiedAccounts: entity.VerifiedAccounts,
		ExcludedTerms:    entity.ExcludedTerms,
	}
}

func (e Entity) names() []string {
	out := make([]string, 0, len(e.Aliases)+1)
	if name := strings.TrimSpace(e.Name); name != "" {
		out = append(out, name)
	}
	for _, alias := range e.Aliases {
		if alias = strings.TrimSpace(alias); alias != "" {
			out = append(out, alias)
		}
	}
	return out
}

// relatedHits counts distinct works, characters and events mentioned in text.
func (e Entity) relatedHits(text string) []string {
	seen := map[string]bool{}
	var hits []string
	groups := [][]string{e.WorkTitles, e.CharacterNames, e.EventTitles}
	for _, group := range groups {
		for _, title := range group {
			title = strings.TrimSpace(title)
			if utf8.RuneCountInString(title) < 2 || seen[title] {
				continue
			}
			if strings.Contains(text, title) {
				seen[title] = true
				hits = append(hits, title)
			}
		}
	}
	return hits
}

func (e Entity) accountVerified(input EvaluateInput) bool {
	if len(e.VerifiedAccounts) == 0 {
		return false
	}
	haystack := strings.ToLower(strings.Join([]string{
		input.Author, input.OriginName, input.OriginPlatform,
	}, " "))
	if strings.TrimSpace(haystack) == "" {
		return false
	}
	for _, account := range e.VerifiedAccounts {
		account = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(account, "@")))
		if account != "" && strings.Contains(haystack, account) {
			return true
		}
	}
	return false
}

func (e Entity) excluded(text string) string {
	for _, term := range e.ExcludedTerms {
		term = strings.TrimSpace(term)
		if term != "" && strings.Contains(text, term) {
			return term
		}
	}
	return ""
}

func countDictionaryMarkers(text string) int {
	hits := 0
	for _, marker := range dictionaryMarkers {
		if strings.Contains(text, marker) {
			hits++
		}
	}
	return hits
}

// EvaluateInput is the per-result data available before insertion.
type EvaluateInput struct {
	Title          string
	Summary        string
	Author         string
	OriginName     string
	OriginPlatform string
	MetadataStatus string
}

// RelevanceResult is the scoring outcome for one search result.
type RelevanceResult struct {
	Score        int
	Label        string
	FilterReason string
	// Reason is a short human-readable explanation, shown in the admin UI and
	// stored in review_reason for auditability.
	Reason string
}

func (r RelevanceResult) Filtered() bool {
	return r.Label == LabelFiltered
}

func filtered(reason, detail string) RelevanceResult {
	return RelevanceResult{
		Label:        LabelFiltered,
		FilterReason: reason,
		Reason:       detail,
	}
}

// Evaluate scores a search result against the verified person entity.
//
// Scoring: full name in title +40, in summary +25, verified account +25,
// each related work/character/event +15 (capped at two signals). Hard
// exclusions never score: dictionary pages, surname-only ambiguity and
// content without enough person context are filtered with a reason.
func Evaluate(entity Entity, input EvaluateInput) RelevanceResult {
	title := strings.TrimSpace(input.Title)
	summary := strings.TrimSpace(input.Summary)
	text := title + " " + summary
	names := entity.names()

	var nameInTitle, nameInBody bool
	for _, name := range names {
		if strings.Contains(title, name) {
			nameInTitle = true
		}
		if strings.Contains(summary, name) {
			nameInBody = true
		}
	}

	if !nameInTitle && !nameInBody {
		if term := entity.excluded(text); term != "" {
			return filtered(FilterDictionaryContent,
				fmt.Sprintf("命中排除词「%s」，判定为姓氏/字典类内容，与人物无关", term))
		}
		if countDictionaryMarkers(text) >= 2 {
			return filtered(FilterDictionaryContent,
				"命中多个字典类特征（拼音/部首/笔画/组词等），判定为汉字解释内容")
		}
		if entity.Name != "" {
			surname := string([]rune(entity.Name)[0])
			if strings.Contains(text, surname) {
				return filtered(FilterAmbiguousEntity,
					fmt.Sprintf("仅命中姓氏「%s」而缺少完整姓名，实体指向不明确", surname))
			}
		}
	}

	var parts []string
	score := 0
	if nameInTitle {
		score += ScoreTitle
		parts = append(parts, "标题包含人物姓名 +40")
	}
	if nameInBody {
		score += ScoreBody
		parts = append(parts, "摘要包含人物姓名 +25")
	}
	if entity.accountVerified(input) {
		score += ScoreVerifiedAccount
		parts = append(parts, "来源为已验证账号 +25")
	}

	related := entity.relatedHits(text)
	for i, title := range related {
		if i >= maxRelatedSignals {
			break
		}
		score += ScoreRelated
		parts = append(parts, fmt.Sprintf("提及关联内容「%s」+15", title))
	}

	lowContext := !nameInTitle && input.MetadataStatus != "ok" &&
		utf8.RuneCountInString(summary) < 20
	if lowContext && score > 0 {
		score -= lowContextPenalty
		if score < 0 {
			score = 0
		}
		parts = append(parts, "缺少原文上下文 -20")
	}

	result := RelevanceResult{Score: score}
	switch {
	case score >= thresholdHigh:
		result.Label = LabelHigh
	case score >= thresholdCandidate:
		result.Label = LabelCandidate
	default:
		result.Label = LabelFiltered
		switch {
		case lowContext:
			result.FilterReason = FilterInsufficientCtx
		case len(related) > 0 || entity.accountVerified(input):
			result.FilterReason = FilterInsufficientCtx
		case len([]rune(text)) >= 8:
			result.FilterReason = FilterIrrelevantPerson
		default:
			result.FilterReason = FilterInsufficientCtx
		}
	}

	if len(parts) == 0 {
		parts = append(parts, "未命中人物姓名、已验证账号或关联内容")
	}
	result.Reason = fmt.Sprintf("相关度 %d 分：%s", score, strings.Join(parts, "；"))
	return result
}

// genericSearchTerms are too broad to identify the person on their own.
var genericSearchTerms = map[string]bool{
	"田": true, "演员": true, "女演员": true, "男演员": true, "女明星": true,
	"明星": true, "艺人": true, "娱乐圈": true, "作品": true, "新剧": true,
	"图片": true, "写真": true, "采访": true, "动态": true,
}

// ValidateSearchQuery rejects searches that cannot identify the person. A task
// query must contain the verified artist name or an alias; generic terms alone
// (for example "田" or "演员") are refused because they pull in surnames,
// dictionary entries and unrelated people.
func ValidateSearchQuery(query string, names []string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return errors.New("搜索词不能为空")
	}
	lower := strings.ToLower(query)
	if genericSearchTerms[lower] {
		return fmt.Errorf("搜索词「%s」过于宽泛，请包含人物姓名或已验证别名", query)
	}

	hasName := false
	for _, name := range names {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		hasName = true
		if strings.Contains(lower, strings.ToLower(name)) {
			return nil
		}
	}
	if !hasName {
		return errors.New("未配置人物姓名或别名，无法校验搜索词")
	}
	return fmt.Errorf("搜索词「%s」必须包含人物姓名或已验证别名", query)
}
