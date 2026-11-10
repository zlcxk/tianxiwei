package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"xiweispace/backend/internal/discovery"
	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

var (
	validProviderTypes = []string{"bing_rss", "searxng"}
	validPrecisions    = []string{"verified", "estimated", "unknown"}
	envNameRE          = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func (s *Server) adminListSearchProviders(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListSearchProviders(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取搜索服务失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func validateProviderInput(name, providerType, endpoint, apiKeyEnv string, config map[string]any) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("搜索服务名称不能为空")
	}
	if !isOneOf(providerType, validProviderTypes...) {
		return fmt.Errorf("搜索服务类型不正确")
	}
	if providerType == "searxng" && strings.TrimSpace(endpoint) == "" {
		return fmt.Errorf("SearXNG 需要填写实例地址")
	}
	if strings.TrimSpace(endpoint) != "" {
		// Admin-configured endpoints may be self-hosted. Only enforce scheme and
		// host syntax here; metadata page fetches use the strict SSRF policy.
		u, err := url.Parse(strings.TrimSpace(endpoint))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			return fmt.Errorf("搜索服务地址必须是有效的 http(s) URL")
		}
	}
	if apiKeyEnv != "" && !envNameRE.MatchString(apiKeyEnv) {
		return fmt.Errorf("API Key 环境变量名只能包含字母、数字和下划线")
	}
	if configHasSecret(config) {
		return fmt.Errorf("搜索服务配置不能包含 API Key、Token、密码等密钥；请使用环境变量")
	}
	return nil
}

func configHasSecret(cfg map[string]any) bool {
	for key, value := range cfg {
		if isSecretKey(key) {
			return true
		}
		if nested, ok := value.(map[string]any); ok && configHasSecret(nested) {
			return true
		}
	}
	return false
}

func (s *Server) adminCreateSearchProvider(w http.ResponseWriter, r *http.Request) {
	var in store.SearchProviderInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.APIKeyEnv = strings.TrimSpace(in.APIKeyEnv)
	if err := validateProviderInput(in.Name, in.ProviderType, in.Endpoint, in.APIKeyEnv, in.Config); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	provider, err := s.store.CreateSearchProvider(r.Context(), in)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "同名搜索服务已存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建搜索服务失败")
		return
	}
	writeJSON(w, http.StatusCreated, provider)
}

func (s *Server) adminUpdateSearchProvider(w http.ResponseWriter, r *http.Request) {
	old, err := s.store.GetSearchProvider(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "搜索服务不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取搜索服务失败")
		return
	}
	var in store.SearchProviderPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	name, typ, endpoint, keyEnv := old.Name, old.ProviderType, old.Endpoint, old.APIKeyEnv
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if in.ProviderType != nil {
		typ = *in.ProviderType
	}
	if in.Endpoint != nil {
		endpoint = strings.TrimSpace(*in.Endpoint)
	}
	if in.APIKeyEnv != nil {
		keyEnv = strings.TrimSpace(*in.APIKeyEnv)
	}
	cfg := old.Config
	if in.Config != nil {
		cfg = in.Config
	}
	if err := validateProviderInput(name, typ, endpoint, keyEnv, cfg); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	provider, err := s.store.UpdateSearchProvider(r.Context(), old.ID, in)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "同名搜索服务已存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "更新搜索服务失败")
		return
	}
	writeJSON(w, http.StatusOK, provider)
}

func (s *Server) adminTestSearchProvider(w http.ResponseWriter, r *http.Request) {
	model, err := s.store.GetSearchProvider(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "搜索服务不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取搜索服务失败")
		return
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result := models.SearchTestResult{}
	provider, err := discovery.NewSearchProvider(model)
	if err == nil {
		err = provider.Validate()
	}
	if err == nil {
		results, searchErr := provider.Search(ctx, discovery.SearchRequest{Query: "田曦薇", Page: 1, PageSize: 3})
		err = searchErr
		result.ResultCount = len(results)
	}
	result.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		result.OK = false
		result.Message = err.Error()
	} else {
		result.OK = true
		result.Message = "连接成功"
	}
	_ = s.store.RecordProviderTest(context.Background(), model.ID, result.OK, int(result.LatencyMS), result.Message)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) adminListSearchTasks(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListSearchTasks(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取搜索任务失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func normalizeDomains(domains []string) ([]string, error) {
	if len(domains) > 10 {
		return nil, fmt.Errorf("站点限制最多 10 个域名")
	}
	out := make([]string, 0, len(domains))
	for _, raw := range domains {
		d := strings.TrimSpace(raw)
		d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
		if i := strings.IndexAny(d, "/?#"); i >= 0 {
			d = d[:i]
		}
		d = strings.TrimSuffix(strings.ToLower(d), ".")
		if d == "" {
			continue
		}
		if strings.ContainsAny(d, " \t\r\n") || !strings.Contains(d, ".") {
			return nil, fmt.Errorf("域名格式不正确：%s", raw)
		}
		out = append(out, d)
	}
	if len(out) > 10 {
		return nil, fmt.Errorf("站点限制最多 10 个域名")
	}
	return out, nil
}

func validateTaskFields(name, query, providerID, hint, sourceType, trust string, interval, maxResults int) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(query) == "" || providerID == "" {
		return fmt.Errorf("任务名称、搜索服务与关键词不能为空")
	}
	if hint != "" && !isOneOf(hint, validContentTypes...) {
		return fmt.Errorf("内容分类不正确")
	}
	if !isOneOf(sourceType, validSourceTypes...) || !isOneOf(trust, validTrustLevels...) {
		return fmt.Errorf("来源类型或可信等级不正确")
	}
	if interval != 0 && interval < 15 {
		return fmt.Errorf("搜索间隔不能小于 15 分钟")
	}
	if maxResults < 0 || maxResults > 50 {
		return fmt.Errorf("单次最大结果数必须在 1–50 之间")
	}
	return nil
}

func (s *Server) adminCreateSearchTask(w http.ResponseWriter, r *http.Request) {
	var in store.SearchTaskInput
	if !decodeJSON(w, r, &in) {
		return
	}
	name := strings.TrimSpace(in.Name)
	query := strings.TrimSpace(in.Query)
	if err := validateTaskFields(name, query, in.ProviderID, in.ContentTypeHint, in.SourceType, in.TrustLevel, in.IntervalMinutes, in.MaxResults); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	domains, err := normalizeDomains(in.Domains)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	in.Name, in.Query, in.Domains = name, query, domains
	if _, err := s.store.GetSearchProvider(r.Context(), in.ProviderID); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", "搜索服务不存在")
		return
	}
	if in.MaxResults == 0 {
		in.MaxResults = 20
	}
	task, err := s.store.CreateSearchTask(r.Context(), in)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "同名搜索任务已存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建搜索任务失败")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) adminUpdateSearchTask(w http.ResponseWriter, r *http.Request) {
	old, err := s.store.GetSearchTask(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "搜索任务不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取搜索任务失败")
		return
	}
	var in store.SearchTaskPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	name, query, providerID := old.Name, old.Query, old.ProviderID
	hint, sourceType, trust := old.ContentTypeHint, old.SourceType, old.TrustLevel
	interval, maxResults := old.IntervalMinutes, old.MaxResults
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
	}
	if in.Query != nil {
		query = strings.TrimSpace(*in.Query)
	}
	if in.ProviderID != nil {
		providerID = *in.ProviderID
	}
	if in.ContentTypeHint != nil {
		hint = *in.ContentTypeHint
	}
	if in.SourceType != nil {
		sourceType = *in.SourceType
	}
	if in.TrustLevel != nil {
		trust = *in.TrustLevel
	}
	if in.IntervalMinutes != nil {
		interval = *in.IntervalMinutes
	}
	if in.MaxResults != nil {
		maxResults = *in.MaxResults
	}
	if err := validateTaskFields(name, query, providerID, hint, sourceType, trust, interval, maxResults); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if in.Domains != nil {
		domains, err := normalizeDomains(*in.Domains)
		if err != nil {
			apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		in.Domains = &domains
	}
	if providerID != old.ProviderID {
		if _, err := s.store.GetSearchProvider(r.Context(), providerID); err != nil {
			apiError(w, http.StatusBadRequest, "invalid_request", "搜索服务不存在")
			return
		}
	}
	task, err := s.store.UpdateSearchTask(r.Context(), old.ID, in)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "同名搜索任务已存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "更新搜索任务失败")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) adminDeleteSearchTask(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteSearchTask(r.Context(), chi.URLParam(r, "id")); errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "搜索任务不存在")
		return
	} else if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "删除搜索任务失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) adminRunSearchTask(w http.ResponseWriter, r *http.Request) {
	runID, err := s.store.QueueSearchRun(r.Context(), chi.URLParam(r, "id"), "manual")
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "搜索任务不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建搜索任务失败")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"run_id": runID})
}

func (s *Server) adminListSearchRuns(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListSearchRuns(r.Context(), 100)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取搜索记录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminListEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListEvents(r.Context(), true)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取事件失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func validateEventStatus(status string) bool {
	return isOneOf(status, "draft", "published", "archived")
}

func (s *Server) adminCreateEvent(w http.ResponseWriter, r *http.Request) {
	var in store.EventInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Status = strings.TrimSpace(in.Status)
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.Title == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "事件标题不能为空")
		return
	}
	if !validateEventStatus(in.Status) {
		apiError(w, http.StatusBadRequest, "invalid_request", "事件状态不正确")
		return
	}
	if in.EventDate != "" {
		if _, err := time.Parse("2006-01-02", in.EventDate); err != nil {
			apiError(w, http.StatusBadRequest, "invalid_request", "事件日期格式应为 YYYY-MM-DD")
			return
		}
	}
	artist, err := s.store.GetArtist(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取艺人资料失败")
		return
	}
	event, err := s.store.CreateEvent(r.Context(), artist.ID, in)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建事件失败")
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) adminUpdateEvent(w http.ResponseWriter, r *http.Request) {
	var in store.EventInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "事件标题不能为空")
		return
	}
	if !validateEventStatus(in.Status) {
		apiError(w, http.StatusBadRequest, "invalid_request", "事件状态不正确")
		return
	}
	if in.EventDate != "" {
		if _, err := time.Parse("2006-01-02", in.EventDate); err != nil {
			apiError(w, http.StatusBadRequest, "invalid_request", "事件日期格式应为 YYYY-MM-DD")
			return
		}
	}
	event, err := s.store.UpdateEvent(r.Context(), chi.URLParam(r, "id"), in)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "事件不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "更新事件失败")
		return
	}
	writeJSON(w, http.StatusOK, event)
}

func (s *Server) adminDeleteEvent(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteEvent(r.Context(), chi.URLParam(r, "id")); errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "事件不存在")
		return
	} else if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "删除事件失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
