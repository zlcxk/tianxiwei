package collector

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"xiweispace/backend/internal/discovery"
	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

type SearchProcessor struct {
	Store   *store.Store
	Fetcher *discovery.MetadataFetcher
}

func NewSearchProcessor(s *store.Store) *SearchProcessor {
	return &SearchProcessor{Store: s, Fetcher: discovery.NewMetadataFetcher()}
}

func (p *SearchProcessor) Run(parent context.Context, run models.SearchRun, task models.SearchTask) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	log := slog.With("search_run_id", run.ID, "task", task.Name)
	providerModel, err := p.Store.GetSearchProvider(ctx, task.ProviderID)
	if err != nil {
		p.fail(context.Background(), run, task, "读取搜索服务失败: "+err.Error(), log)
		return
	}
	if !providerModel.Enabled {
		p.fail(context.Background(), run, task, "搜索服务已停用", log)
		return
	}
	provider, err := discovery.NewSearchProvider(providerModel)
	if err != nil {
		p.fail(context.Background(), run, task, err.Error(), log)
		return
	}
	if err := provider.Validate(); err != nil {
		p.fail(context.Background(), run, task, err.Error(), log)
		return
	}
	rules, err := p.Store.ListRules(ctx)
	if err != nil {
		p.fail(context.Background(), run, task, "加载分类规则失败: "+err.Error(), log)
		return
	}

	var since *time.Time
	if task.LastSuccessAt != nil {
		// Overlap one day so late-indexed results are not lost. URL dedupe keeps
		// repeated results from being inserted twice.
		t := task.LastSuccessAt.Add(-24 * time.Hour)
		since = &t
	}
	until := time.Now()
	pageSize := 10
	pages := (task.MaxResults + pageSize - 1) / pageSize
	if pages > 5 {
		pages = 5
	}

	results := make([]discovery.SearchResult, 0, task.MaxResults)
	for page := 1; page <= pages && len(results) < task.MaxResults; page++ {
		pageResults, err := provider.Search(ctx, discovery.SearchRequest{
			Query: task.Query, Domains: task.Domains, Since: since, Until: &until,
			Page: page, PageSize: pageSize,
		})
		if err != nil {
			if page == 1 {
				p.fail(context.Background(), run, task, "搜索失败: "+err.Error(), log)
				return
			}
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

	fetched, inserted, duplicates, failed, parsedOK, parseFailed := len(results), 0, 0, 0, 0, 0
	seen := map[string]bool{}
	itemErrors := make([]string, 0, 3)
	for _, result := range results {
		if ctx.Err() != nil {
			failed++
			continue
		}
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

		originName, platform, originType := discovery.PlatformForURL(canonical)
		if result.SourceName != "" {
			originName = result.SourceName
		}
		if result.Platform != "" {
			platform = result.Platform
		}
		meta := discovery.PageMetadata{}
		metaStatus := "not_attempted"
		metaError := ""
		fctx, fcancel := context.WithTimeout(ctx, 7*time.Second)
		meta, err := p.Fetcher.Fetch(fctx, canonical)
		fcancel()
		switch {
		case err == nil:
			metaStatus = "ok"
			parsedOK++
		case errors.Is(err, discovery.ErrBlockedResponse) || errors.Is(err, discovery.ErrBlockedURL):
			metaStatus = "blocked"
			metaError = truncateRunes(err.Error(), 300)
			parseFailed++
		default:
			metaStatus = "failed"
			metaError = truncateRunes(err.Error(), 300)
			parseFailed++
		}
		if err != nil {
			log.Debug("metadata unavailable", "url", canonical, "error", err)
		}

		if meta.Title != "" {
			title = meta.Title
		}
		if meta.Description != "" {
			summary = meta.Description
		}
		author := meta.Author
		if author == "" {
			author = originName
		}

		published := meta.PublishedAt
		precision := "verified"
		if published == nil && result.PublishedAt != nil {
			published = result.PublishedAt
			precision = "estimated"
		}
		if published == nil {
			precision = "unknown"
		}

		pseudoSource := models.Source{SourceType: originType, TrustLevel: task.TrustLevel}
		contentType := task.ContentTypeHint
		if contentType == "" {
			contentType = InferContentType(pseudoSource, title+" "+summary)
			contentType = searchContentType(contentType, originType)
		}
		classification := Classify(title+" "+summary, rules)
		reason := "搜索发现候选：发布时间或摘要需人工核验原文"
		if precision == "unknown" {
			reason = "搜索发现候选：原文未提供可信发布时间，需人工核验"
		}
		if metaStatus == "blocked" {
			reason += "；目标页面限制自动读取"
		}
		raw, _ := json.Marshal(map[string]any{
			"discovery": map[string]any{
				"provider":            result.Provider,
				"search_task_id":      task.ID,
				"search_task_name":    task.Name,
				"query":               task.Query,
				"search_title":        result.Title,
				"search_snippet":      result.Snippet,
				"search_published_at": result.PublishedAt,
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
			Author:             truncateRunes(author, 120),
			URL:                canonical,
			CoverURL:           "",
			URLHash:            urlHash,
			PublishedAt:        published,
			PublishedPrecision: precision,
			OriginName:         originName,
			OriginPlatform:     platform,
			OriginType:         originType,
			MetadataStatus:     metaStatus,
			MetadataError:      metaError,
			TrustLevel:         task.TrustLevel,
			IsOfficial:         originType == "official" || task.TrustLevel == "official",
			ReviewStatus:       "pending",
			ContentStatus:      "pending",
			ReviewReason:       reason,
			RawData:            raw,
			SceneIDs:           classification.SceneIDs,
		})
		if err != nil {
			failed++
			log.Warn("insert search candidate failed", "url", canonical, "error", err)
			if len(itemErrors) < 2 {
				itemErrors = append(itemErrors, err.Error())
			}
			continue
		}
		if status == store.Duplicate {
			duplicates++
		} else {
			inserted++
		}
	}

	errMsg := ""
	if failed > 0 {
		errMsg = "部分搜索结果处理失败: " + strings.Join(itemErrors, "; ")
		if len(itemErrors) == 0 {
			errMsg = "部分搜索结果缺少标题、链接或处理超时"
		}
	}
	if err := p.Store.CompleteSearchRun(context.Background(), run.ID, task.ID,
		fetched, inserted, duplicates, failed, parsedOK, parseFailed, errMsg, false); err != nil {
		log.Error("complete search run", "error", err)
		return
	}
	log.Info("search finished", "fetched", fetched, "inserted", inserted, "duplicates", duplicates,
		"failed", failed, "metadata_ok", parsedOK, "metadata_failed", parseFailed)
}

func (p *SearchProcessor) fail(ctx context.Context, run models.SearchRun, task models.SearchTask, message string, log *slog.Logger) {
	log.Warn("search run failed", "error", message)
	if err := p.Store.CompleteSearchRun(ctx, run.ID, task.ID, 0, 0, 0, 0, 0, 0, message, true); err != nil {
		log.Error("complete failed search run", "error", err)
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
