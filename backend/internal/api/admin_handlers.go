package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"xiweispace/backend/internal/collector"
	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

var (
	validSourceTypes     = []string{"official", "media", "community", "reference", "internal"}
	validTrustLevels     = []string{"official", "media", "community", "reference", "internal"}
	validContentTypes    = []string{"social", "post", "news", "drama", "variety", "media", "video", "interview", "image", "fashion", "event", "work", "official", "topic", "internal", "other"}
	validContentStatuses = []string{"pending", "reviewing", "published", "rejected", "expired", "archived"}
	validReviewStatuses  = []string{"none", "pending", "approved", "rejected"}
)

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	o, err := s.store.GetOverview(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取概览失败")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) adminDataHealth(w http.ResponseWriter, r *http.Request) {
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && !isOneOf(filter, "image", "summary", "date", "relation", "source_url") {
		apiError(w, http.StatusBadRequest, "invalid_request", "筛选条件不正确")
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			apiError(w, http.StatusBadRequest, "invalid_request", "数量不正确")
			return
		}
		limit = n
	}
	health, err := s.store.DataHealth(r.Context(), filter, limit)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取资料完善度失败")
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) adminListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取数据源失败")
		return
	}
	out := make([]models.Source, 0, len(sources))
	for _, src := range sources {
		src.Config = maskConfig(src.Config)
		out = append(out, src)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) adminCreateSource(w http.ResponseWriter, r *http.Request) {
	var in store.SourceInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "来源名称不能为空")
		return
	}
	if !isOneOf(in.AdapterType, "rss", "manual") {
		apiError(w, http.StatusBadRequest, "invalid_request", "采集方式不正确")
		return
	}
	if !isOneOf(in.SourceType, validSourceTypes...) || !isOneOf(in.TrustLevel, validTrustLevels...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "来源类型或可信等级不正确")
		return
	}
	if in.AdapterType == "rss" {
		if _, err := collector.NewAdapter(models.Source{Name: in.Name, AdapterType: "rss", Config: in.Config}); err != nil {
			apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
	}
	src, err := s.store.CreateSource(r.Context(), in)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "同名数据源已存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建数据源失败")
		return
	}
	src.Config = maskConfig(src.Config)
	writeJSON(w, http.StatusCreated, src)
}

func (s *Server) adminUpdateSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	old, err := s.store.GetSource(r.Context(), id)
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "数据源不存在")
		return
	}
	var in store.SourcePatch
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.SourceType != nil && !isOneOf(*in.SourceType, validSourceTypes...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "来源类型不正确")
		return
	}
	if in.TrustLevel != nil && !isOneOf(*in.TrustLevel, validTrustLevels...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "可信等级不正确")
		return
	}
	if in.AdapterType != nil && !isOneOf(*in.AdapterType, "rss", "manual") {
		apiError(w, http.StatusBadRequest, "invalid_request", "采集方式不正确")
		return
	}
	if in.Config != nil {
		in.Config = restoreMaskedConfig(in.Config, old.Config)
		adapterType := old.AdapterType
		if in.AdapterType != nil {
			adapterType = *in.AdapterType
		}
		if adapterType == "rss" {
			if _, err := collector.NewAdapter(models.Source{Name: old.Name, AdapterType: "rss", Config: in.Config}); err != nil {
				apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
		}
	}
	src, err := s.store.UpdateSource(r.Context(), id, in)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "更新数据源失败")
		return
	}
	src.Config = maskConfig(src.Config)
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) adminRunSource(w http.ResponseWriter, r *http.Request) {
	src, err := s.store.GetSource(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "数据源不存在")
		return
	}
	if src.AdapterType == "manual" {
		apiError(w, http.StatusBadRequest, "manual_source", "手动来源不支持自动采集，请使用内容手动录入")
		return
	}
	jobID, err := s.store.QueueJob(r.Context(), src.ID, "manual")
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建采集任务失败")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (s *Server) adminListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.ListJobs(r.Context(), 100, strings.TrimSpace(r.URL.Query().Get("status")))
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取采集任务失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": jobs})
}

func (s *Server) adminRetryJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := s.store.RetryJob(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "任务不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "重试失败")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

func (s *Server) adminListReviews(w http.ResponseWriter, r *http.Request) {
	items, _, err := s.store.ListContents(r.Context(), store.ContentFilter{
		IncludeUnpublished: true,
		ReviewStatus:       "pending",
		Limit:              100,
	})
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取待审核内容失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminApprove(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	c, err := s.store.SetReviewStatus(r.Context(), chi.URLParam(r, "id"), u.ID, true, "")
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "审核操作失败")
		return
	}
	c.RawData = nil
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) adminReject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength > 0 {
		if !decodeJSON(w, r, &in) {
			return
		}
	}
	if strings.TrimSpace(in.Reason) == "" {
		in.Reason = "未通过审核"
	}
	u := userFrom(r.Context())
	c, err := s.store.SetReviewStatus(r.Context(), chi.URLParam(r, "id"), u.ID, false, in.Reason)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "审核操作失败")
		return
	}
	c.RawData = nil
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) adminListContents(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := pagination(r)
	q := r.URL.Query()
	var contentTypes []string
	for _, ct := range strings.Split(q.Get("content_type"), ",") {
		if v := strings.TrimSpace(ct); v != "" {
			contentTypes = append(contentTypes, v)
		}
	}
	filter := store.ContentFilter{
		Keyword:            strings.TrimSpace(q.Get("keyword")),
		SourceType:         strings.TrimSpace(q.Get("source_type")),
		SourceQuery:        strings.TrimSpace(q.Get("source")),
		ContentTypes:       contentTypes,
		IncludeUnpublished: true,
		ContentStatus:      strings.TrimSpace(q.Get("content_status")),
		ReviewStatus:       strings.TrimSpace(q.Get("review_status")),
		OnlyCandidates:     q.Get("candidate") == "1" || q.Get("candidate") == "true",
		SearchTaskID:       strings.TrimSpace(q.Get("search_task_id")),
		MetadataStatus:     strings.TrimSpace(q.Get("metadata_status")),
		FilterReason:       strings.TrimSpace(q.Get("filter_reason")),
		RelevanceLabel:     strings.TrimSpace(q.Get("relevance_label")),
		EventID:            strings.TrimSpace(q.Get("event_id")),
		Sort:               "latest",
		Limit:              pageSize,
		Offset:             offset,
	}
	items, total, err := s.store.ListContents(r.Context(), filter)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取内容失败")
		return
	}
	writeJSON(w, http.StatusOK, models.Paged[models.Content]{
		Items: items, Page: page, PageSize: pageSize, Total: total,
	})
}

func (s *Server) adminCreateContent(w http.ResponseWriter, r *http.Request) {
	var in store.ContentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Title) == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "标题不能为空")
		return
	}
	if in.ContentType != "" && !isOneOf(in.ContentType, validContentTypes...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "内容类型不正确")
		return
	}
	artist, err := s.store.GetArtist(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取艺人资料失败")
		return
	}
	in.ArtistID = artist.ID
	if in.SourceID != "" {
		src, err := s.store.GetSource(r.Context(), in.SourceID)
		if err != nil {
			apiError(w, http.StatusBadRequest, "invalid_request", "数据源不存在")
			return
		}
		if in.SourceAuthor == "" {
			in.SourceAuthor = src.Name
		}
		if in.TrustLevel == "" {
			in.TrustLevel = src.TrustLevel
		}
		in.IsOfficial = in.IsOfficial || src.SourceType == "official"
	}
	if in.SourceAuthor == "" {
		in.SourceAuthor = "曦薇空间编辑部"
	}
	u := userFrom(r.Context())
	c, err := s.store.CreateContent(r.Context(), in, u.ID)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "duplicate_url", "相同原始链接的内容已存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建内容失败")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) adminGetContent(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.GetContentAdmin(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取内容失败")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) adminUpdateContent(w http.ResponseWriter, r *http.Request) {
	var in store.ContentPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.ContentType != nil && !isOneOf(*in.ContentType, validContentTypes...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "内容类型不正确")
		return
	}
	if in.TrustLevel != nil && !isOneOf(*in.TrustLevel, validTrustLevels...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "可信等级不正确")
		return
	}
	if in.ContentStatus != nil && !isOneOf(*in.ContentStatus, validContentStatuses...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "发布状态不正确")
		return
	}
	if in.ReviewStatus != nil && !isOneOf(*in.ReviewStatus, validReviewStatuses...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "审核状态不正确")
		return
	}
	if in.PublishedPrecision != nil && !isOneOf(*in.PublishedPrecision, validPrecisions...) {
		apiError(w, http.StatusBadRequest, "invalid_request", "发布时间精度不正确")
		return
	}
	if in.ClearPublishedAt && in.PublishedAt != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", "不能同时设置和清除发布时间")
		return
	}
	if in.PublishedAt != nil && in.PublishedPrecision == nil {
		precision := "verified"
		in.PublishedPrecision = &precision
	}
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "标题不能为空")
		return
	}
	u := userFrom(r.Context())
	c, err := s.store.UpdateContent(r.Context(), chi.URLParam(r, "id"), u.ID, in, "管理员编辑内容")
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "duplicate_url", "相同原始链接的内容已存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "更新内容失败")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) adminGetArtist(w http.ResponseWriter, r *http.Request) {
	s.getArtist(w, r)
}

func (s *Server) adminUpdateArtist(w http.ResponseWriter, r *http.Request) {
	artist, err := s.store.GetArtist(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取艺人资料失败")
		return
	}
	var in store.ArtistInput
	if !decodeJSON(w, r, &in) {
		return
	}
	updated, err := s.store.UpdateArtist(r.Context(), artist.ID, in)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// adminGetArtistEntity returns the editorial person entity used for relevance
// scoring. Aliases and verified accounts are never inferred automatically.
func (s *Server) adminGetArtistEntity(w http.ResponseWriter, r *http.Request) {
	entity, err := s.store.GetArtistEntity(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取人物实体配置失败")
		return
	}
	writeJSON(w, http.StatusOK, entity)
}

func (s *Server) adminPutArtistEntity(w http.ResponseWriter, r *http.Request) {
	var in models.ArtistEntity
	if !decodeJSON(w, r, &in) {
		return
	}
	entity, err := s.store.PutArtistEntity(r.Context(), in)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entity)
}

func (s *Server) adminListWorks(w http.ResponseWriter, r *http.Request) {
	artist, err := s.store.GetArtist(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取艺人资料失败")
		return
	}
	works, err := s.store.ListWorks(r.Context(), artist.ID, strings.TrimSpace(r.URL.Query().Get("type")))
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取作品失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": works})
}

func (s *Server) adminCreateWork(w http.ResponseWriter, r *http.Request) {
	var in store.WorkInput
	if !decodeJSON(w, r, &in) {
		return
	}
	artist, err := s.store.GetArtist(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取艺人资料失败")
		return
	}
	work, err := s.store.CreateWork(r.Context(), artist.ID, in)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, work)
}

func (s *Server) adminUpdateWork(w http.ResponseWriter, r *http.Request) {
	var in store.WorkInput
	if !decodeJSON(w, r, &in) {
		return
	}
	work, err := s.store.UpdateWork(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func (s *Server) adminDeleteWork(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteWork(r.Context(), chi.URLParam(r, "id")); err != nil {
		apiError(w, http.StatusNotFound, "not_found", "作品不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) adminListCharacters(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCharacters(r.Context(), strings.TrimSpace(r.URL.Query().Get("work_id")))
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取角色失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminCreateCharacter(w http.ResponseWriter, r *http.Request) {
	var in store.CharacterInput
	if !decodeJSON(w, r, &in) {
		return
	}
	c, err := s.store.CreateCharacter(r.Context(), in)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) adminUpdateCharacter(w http.ResponseWriter, r *http.Request) {
	var in store.CharacterInput
	if !decodeJSON(w, r, &in) {
		return
	}
	c, err := s.store.UpdateCharacter(r.Context(), chi.URLParam(r, "id"), in)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) adminDeleteCharacter(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCharacter(r.Context(), chi.URLParam(r, "id")); err != nil {
		apiError(w, http.StatusNotFound, "not_found", "角色不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) adminListScenes(w http.ResponseWriter, r *http.Request) {
	scenes, err := s.store.ListAllScenes(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取场景失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": scenes})
}

func (s *Server) adminUpdateScene(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Enabled     *bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := s.store.UpdateScene(r.Context(), chi.URLParam(r, "id"), in.Name, in.Description, in.Enabled); err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "更新场景失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) adminListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListRules(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取分类规则失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rules})
}

func (s *Server) adminCreateRule(w http.ResponseWriter, r *http.Request) {
	var in store.RuleInput
	if !decodeJSON(w, r, &in) {
		return
	}
	id, err := s.store.CreateRule(r.Context(), in)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "该场景已存在相同关键词规则")
		return
	}
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *Server) adminUpdateRule(w http.ResponseWriter, r *http.Request) {
	var in store.RuleInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := s.store.UpdateRule(r.Context(), chi.URLParam(r, "id"), in); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) adminDeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteRule(r.Context(), chi.URLParam(r, "id")); err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "删除规则失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) adminGetRecommendWeights(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetRecommendWeights(r.Context()))
}

func (s *Server) adminPutRecommendWeights(w http.ResponseWriter, r *http.Request) {
	var in store.RecommendWeights
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := s.store.PutRecommendWeights(r.Context(), in); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func maskConfig(cfg map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range cfg {
		if nested, ok := v.(map[string]any); ok {
			out[k] = maskConfig(nested)
			continue
		}
		if isSecretKey(k) {
			if s, ok := v.(string); ok && s != "" {
				out[k] = "••••••••"
				continue
			}
		}
		out[k] = v
	}
	return out
}

func restoreMaskedConfig(newCfg, oldCfg map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range newCfg {
		if s, ok := v.(string); ok && s == "••••••••" {
			if old, exists := oldCfg[k]; exists {
				out[k] = old
				continue
			}
		}
		if nested, ok := v.(map[string]any); ok {
			if oldNested, ok := oldCfg[k].(map[string]any); ok {
				out[k] = restoreMaskedConfig(nested, oldNested)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func isSecretKey(key string) bool {
	k := strings.ToLower(key)
	for _, part := range []string{"token", "secret", "password", "cookie", "authorization", "apikey", "api_key", "access_key"} {
		if strings.Contains(k, part) {
			return true
		}
	}
	return false
}
