package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"xiweispace/backend/internal/discovery"
	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

// DefaultSearchOverlap is the re-scan window applied before the last complete
// success, so late-indexed results are not lost. URL dedupe keeps repeated
// results from being inserted twice.
const DefaultSearchOverlap = 7 * 24 * time.Hour

// Metadata fetches are bounded independently of the task deadline: at most
// four pages are read at once, each with a short per-page timeout.
const (
	metadataConcurrency  = 4
	metadataFetchTimeout = 7 * time.Second
)

type SearchProcessor struct {
	Store    *store.Store
	Fetcher  *discovery.MetadataFetcher
	WorkerID string
	// OverlapWindow may be configured from SEARCH_OVERLAP_DAYS. It is measured
	// back from the last fully successful run, never from a partial or failed
	// run.
	OverlapWindow time.Duration
}

func NewSearchProcessor(s *store.Store) *SearchProcessor {
	return &SearchProcessor{
		Store:         s,
		Fetcher:       discovery.NewMetadataFetcher(),
		OverlapWindow: DefaultSearchOverlap,
	}
}

// searchSince computes the lower bound for a search request. No lower bound is
// applied until the task has completed at least one fully successful run.
func searchSince(lastSuccess *time.Time, overlap time.Duration) *time.Time {
	if lastSuccess == nil {
		return nil
	}
	if overlap <= 0 {
		overlap = DefaultSearchOverlap
	}
	t := lastSuccess.Add(-overlap)
	return &t
}

// resolveSourceTrust derives the stored trust level from the identified host.
// A task-level label is only a conservative fallback for unrecognized hosts:
// it can never promote a result to official and can never override a host that
// was positively identified as official media, community or reference.
func resolveSourceTrust(identity discovery.SourceIdentity, taskTrust string) string {
	if identity.Known {
		switch identity.SourceType {
		case "official", "media", "community", "reference":
			return identity.SourceType
		}
		return "media"
	}
	switch strings.TrimSpace(taskTrust) {
	case "media", "community", "reference":
		return taskTrust
	default:
		return "media"
	}
}

type pageFetcher interface {
	Fetch(context.Context, string) (discovery.PageMetadata, error)
}

type metadataOutcome struct {
	meta   discovery.PageMetadata
	status string
	errMsg string
	err    error
}

// fetchMetadataBatch reads page metadata with a bounded worker pool. Results
// are written by index, so output order always matches input order even though
// fetches complete out of order. Every candidate keeps its own timeout, and
// cancellation marks unprocessed candidates as not attempted.
func fetchMetadataBatch(ctx context.Context, fetcher pageFetcher, urls []string) []metadataOutcome {
	outcomes := make([]metadataOutcome, len(urls))
	for i := range outcomes {
		outcomes[i].status = "not_attempted"
	}
	jobQueue := make(chan int)
	workers := metadataConcurrency
	if workers > len(urls) {
		workers = len(urls)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobQueue {
				if ctx.Err() != nil {
					outcomes[index].status = "not_attempted"
					outcomes[index].err = ctx.Err()
					continue
				}
				fctx, fcancel := context.WithTimeout(ctx, metadataFetchTimeout)
				meta, err := fetcher.Fetch(fctx, urls[index])
				fcancel()
				outcomes[index].meta = meta
				outcomes[index].err = err
				switch {
				case err == nil:
					outcomes[index].status = "ok"
				case errors.Is(err, discovery.ErrBlockedResponse) || errors.Is(err, discovery.ErrBlockedURL):
					outcomes[index].status = "blocked"
					outcomes[index].errMsg = truncateRunes(err.Error(), 300)
				default:
					outcomes[index].status = "failed"
					outcomes[index].errMsg = truncateRunes(err.Error(), 300)
				}
			}
		}()
	}
	for index := range urls {
		select {
		case jobQueue <- index:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobQueue)
	wg.Wait()
	return outcomes
}

func (p *SearchProcessor) Run(parent context.Context, run models.SearchRun, task models.SearchTask) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	stopHeartbeat := startHeartbeat(p.WorkerID, "search", func(ctx context.Context) error {
		return p.Store.HeartbeatSearchRun(ctx, run.ID)
	})
	defer stopHeartbeat()

	log := slog.With("worker_id", p.WorkerID, "task_type", "search",
		"run_id", run.ID, "search_task_id", task.ID, "task", task.Name)
	duration := func() string { return time.Since(started).Round(time.Millisecond).String() }

	fail := func(message string) {
		log.Warn("search run failed", "error", message)
		if err := p.Store.CompleteSearchRun(context.Background(), run.ID, task.ID,
			0, 0, 0, 0, 0, 0, message, true, false); err != nil {
			log.Error("complete failed search run", "error", err)
		}
		log.Error("task execution failed", "status", "failed", "duration", duration(),
			"success_count", 0, "failure_count", 0, "error", message)
	}

	providerModel, err := p.Store.GetSearchProvider(ctx, task.ProviderID)
	if err != nil {
		fail("读取搜索服务失败: " + err.Error())
		return
	}
	if !providerModel.Enabled {
		fail("搜索服务已停用")
		return
	}
	provider, err := discovery.NewSearchProvider(providerModel)
	if err != nil {
		fail(err.Error())
		return
	}
	if err := provider.Validate(); err != nil {
		fail(err.Error())
		return
	}
	rules, err := p.Store.ListRules(ctx)
	if err != nil {
		fail("加载分类规则失败: " + err.Error())
		return
	}
	entityCtx, err := p.Store.LoadEntityContext(ctx)
	if err != nil {
		fail("加载人物实体配置失败: " + err.Error())
		return
	}
	entity := EntityFromContext(entityCtx)

	since := searchSince(task.LastSuccessAt, p.OverlapWindow)
	until := time.Now()
	pageSize := 10
	pages := (task.MaxResults + pageSize - 1) / pageSize
	if pages > 5 {
		pages = 5
	}

	var pageFailure string
	results := make([]discovery.SearchResult, 0, task.MaxResults)
	for page := 1; page <= pages && len(results) < task.MaxResults; page++ {
		pageResults, err := provider.Search(ctx, discovery.SearchRequest{
			Query: task.Query, Domains: task.Domains, Since: since, Until: &until,
			Page: page, PageSize: pageSize,
		})
		if err != nil {
			if page == 1 {
				// Without the first page there is nothing usable at all.
				fail("搜索失败: " + err.Error())
				return
			}
			// Later pages may still leave usable earlier results, but the run is
			// incomplete and must be recorded as partial rather than success.
			pageFailure = fmt.Sprintf("第 %d 页搜索失败: %v", page, err)
			log.Warn("search page failed", "page", page, "error", err)
			break
		}
		if len(pageResults) == 0 {
			break
		}
		results = append(results, pageResults...)
	}
	if len(results) > task.MaxResults {
		results = results[:task.MaxResults]
	}

	fetched := len(results)
	inserted, duplicates, failed, parsedOK, parseFailed := 0, 0, 0, 0, 0
	filteredCount, lowConfidenceCount := 0, 0

	type candidate struct {
		result    discovery.SearchResult
		title     string
		summary   string
		canonical string
		urlHash   string
	}
	candidates := make([]candidate, 0, len(results))
	seen := map[string]bool{}
	for _, result := range results {
		title := stripHTML(result.Title)
		summary := stripHTML(result.Snippet)
		canonical := canonicalURL(result.URL)
		if title == "" || canonical == "" {
			failed++
			continue
		}
		if _, err := discovery.ValidatePublicURL(canonical); err != nil {
			failed++
			log.Warn("unsafe search result skipped", "url", result.URL, "error", err)
			continue
		}
		urlHash := hashString(canonical)
		if seen[urlHash] {
			duplicates++
			continue
		}
		seen[urlHash] = true
		candidates = append(candidates, candidate{
			result: result, title: title, summary: summary,
			canonical: canonical, urlHash: urlHash,
		})
	}

	// Fetch page metadata concurrently. Every candidate has an independent
	// timeout and writes to its own index, so the result order stays identical
	// to the search order and no URL is fetched twice.
	urls := make([]string, len(candidates))
	for i := range candidates {
		urls[i] = candidates[i].canonical
	}
	outcomes := fetchMetadataBatch(ctx, p.Fetcher, urls)

	itemErrors := make([]string, 0, 3)
	for i, current := range candidates {
		if ctx.Err() != nil {
			failed++
			continue
		}
		outcome := outcomes[i]
		meta := outcome.meta
		metaStatus := outcome.status
		metaError := outcome.errMsg
		switch metaStatus {
		case "ok":
			parsedOK++
		case "blocked", "failed":
			parseFailed++
		}
		if outcome.err != nil {
			log.Debug("metadata unavailable", "url", current.canonical, "error", outcome.err)
		}

		// Identify the publisher from each result URL. The task configuration is
		// never used as evidence of official identity and never overwrites what
		// the host itself proves.
		identity := discovery.IdentifySource(current.canonical)
		originName, platform, originType := identity.Name, identity.Platform, identity.SourceType
		if originName == "" {
			originName = current.result.SourceName
		}
		if platform == "" {
			platform = current.result.Platform
		}
		trustLevel := resolveSourceTrust(identity, task.TrustLevel)
		// is_official marks an official-source page (currently the recognized
		// official-media domains). It does not mean the artist's verified account;
		// that requires exact account identity matching in relevance scoring.
		isOfficial := identity.Known && identity.SourceType == "official"

		title, summary := current.title, current.summary
		if meta.Title != "" {
			title = meta.Title
		}
		if meta.Description != "" {
			summary = meta.Description
		}
		// Only the account reported by the page itself may count as account
		// identity. The stored author still falls back to the source name.
		accountName := strings.TrimSpace(meta.Author)
		storedAuthor := accountName
		if storedAuthor == "" {
			storedAuthor = originName
		}

		published := meta.PublishedAt
		precision := "verified"
		if published == nil && current.result.PublishedAt != nil {
			published = current.result.PublishedAt
			precision = "estimated"
		}
		if published == nil {
			precision = "unknown"
		}

		contentType := resolveSearchContentType(identity, title, summary, task.ContentTypeHint)
		relevance := Evaluate(entity, EvaluateInput{
			Title:          title,
			Summary:        summary,
			Author:         accountName,
			OriginName:     originName,
			OriginPlatform: platform,
			MetadataStatus: metaStatus,
		})
		classification := Classify(title+" "+summary, rules)
		reason := "搜索发现候选：发布时间或摘要需人工核验原文"
		if precision == "unknown" {
			reason = "搜索发现候选：原文未提供可信发布时间，需人工核验"
		}
		if metaStatus == "blocked" {
			reason += "；目标页面限制自动读取"
		}
		// Explicitly irrelevant results are rejected immediately. Candidates
		// whose context is merely insufficient stay pending for manual review.
		reviewStatus, contentStatus := "pending", "pending"
		switch {
		case relevance.Filtered():
			reviewStatus, contentStatus = "rejected", "rejected"
			reason = "自动过滤：" + relevance.Reason
			if metaStatus == "blocked" {
				reason += "；目标页面限制自动读取"
			}
		case relevance.LowConfidence():
			reason = "低置信候选（上下文不足，待人工核验）：" + relevance.Reason + "；" + reason
		default:
			reason = relevance.Reason + "；" + reason
		}
		raw, _ := json.Marshal(map[string]any{
			"discovery": map[string]any{
				"provider":            current.result.Provider,
				"search_task_id":      task.ID,
				"search_task_name":    task.Name,
				"query":               task.Query,
				"search_title":        current.result.Title,
				"search_snippet":      current.result.Snippet,
				"search_published_at": current.result.PublishedAt,
				"domains":             task.Domains,
			},
			"metadata": map[string]any{
				"status":        metaStatus,
				"error":         metaError,
				"title":         meta.Title,
				"description":   meta.Description,
				"site_name":     meta.SiteName,
				"author":        meta.Author,
				"image_url":     meta.ImageURL,
				"published_at":  meta.PublishedAt,
				"canonical_url": meta.CanonicalURL,
				"final_url":     meta.FinalURL,
			},
		})

		status, err := p.Store.InsertDiscovered(ctx, store.DiscoveredContent{
			SearchTaskID:       task.ID,
			Title:              truncateRunes(title, 300),
			Summary:            truncateRunes(summary, 1200),
			ContentType:        contentType,
			Author:             truncateRunes(storedAuthor, 120),
			URL:                current.canonical,
			CoverURL:           resolveSearchCoverURL(meta.ImageURL),
			URLHash:            current.urlHash,
			ContentHash:        contentFingerprint(title, summary),
			PublishedAt:        published,
			PublishedPrecision: precision,
			OriginName:         originName,
			OriginPlatform:     platform,
			OriginType:         originType,
			MetadataStatus:     metaStatus,
			MetadataError:      metaError,
			TrustLevel:         trustLevel,
			IsOfficial:         isOfficial,
			ReviewStatus:       reviewStatus,
			ContentStatus:      contentStatus,
			ReviewReason:       reason,
			RelevanceScore:     relevance.Score,
			RelevanceLabel:     relevance.Label,
			FilterReason:       relevance.FilterReason,
			RawData:            raw,
			SceneIDs:           classification.SceneIDs,
		})
		if err != nil {
			failed++
			log.Warn("insert search candidate failed", "url", current.canonical, "error", err)
			if len(itemErrors) < 2 {
				itemErrors = append(itemErrors, err.Error())
			}
			continue
		}
		if status == store.Duplicate {
			duplicates++
		} else {
			inserted++
			if relevance.Filtered() {
				filteredCount++
			}
			if relevance.LowConfidence() {
				lowConfidenceCount++
			}
		}
	}

	failures := make([]string, 0, 2)
	if pageFailure != "" {
		failures = append(failures, pageFailure)
	}
	if failed > 0 {
		itemFailure := "部分搜索结果处理失败: " + strings.Join(itemErrors, "; ")
		if len(itemErrors) == 0 {
			itemFailure = "部分搜索结果缺少标题、链接或处理超时"
		}
		failures = append(failures, itemFailure)
	}
	errMsg := strings.Join(failures, "；")
	partial := len(failures) > 0
	status := "success"
	if partial {
		status = "partial"
	}
	if err := p.Store.CompleteSearchRun(context.Background(), run.ID, task.ID,
		fetched, inserted, duplicates, failed, parsedOK, parseFailed, errMsg, false, partial); err != nil {
		log.Error("complete search run", "error", err)
		return
	}
	log.Info("search finished", "status", status,
		"fetched", fetched, "inserted", inserted, "duplicates", duplicates,
		"failed", failed, "filtered", filteredCount, "low_confidence", lowConfidenceCount,
		"metadata_ok", parsedOK, "metadata_failed", parseFailed,
		"success_count", inserted, "failure_count", failed, "duration", duration())
}

// resolveSearchCoverURL accepts only public http(s) image URLs parsed from the
// article page. Invalid schemes, credentials, internal hosts and non-standard
// ports are discarded, so a candidate is still inserted without a cover: an
// unusable image never fails content insertion or the search run.
func resolveSearchCoverURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := discovery.ValidatePublicURL(raw)
	if err != nil {
		return ""
	}
	return u.String()
}

// resolveSearchContentType prefers signals from the actual result page, then
// from the recognized platform, and only falls back to the task hint when the
// result itself offers no type signal.
func resolveSearchContentType(identity discovery.SourceIdentity, title, summary, hint string) string {
	if inferred, ok := inferContentTypeFromText(title + " " + summary); ok {
		return searchContentType(inferred, identity.SourceType)
	}
	if identity.Known {
		switch identity.Name {
		case "哔哩哔哩", "抖音":
			return "video"
		case "微博", "小红书", "知乎", "微信公众号":
			return "post"
		case "百科资料", "百度":
			return "other"
		case "官方媒体", "搜狐", "新浪", "网易", "腾讯新闻", "凤凰网", "澎湃新闻":
			return "news"
		}
	}
	if hint := strings.TrimSpace(hint); hint != "" {
		return hint
	}
	switch identity.SourceType {
	case "community":
		return "post"
	case "official":
		return "news"
	case "reference":
		return "other"
	default:
		return "other"
	}
}

func searchContentType(inferred, originType string) string {
	switch inferred {
	case "media":
		return "news"
	case "social":
		if originType == "community" {
			return "post"
		}
		return "news"
	case "other":
		if originType == "community" {
			return "post"
		}
		if originType == "reference" {
			return "other"
		}
		return "news"
	default:
		return inferred
	}
}
