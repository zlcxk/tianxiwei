package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (s *Server) adminListEditorialPicks(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListEditorialPicks(r.Context(), true)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取编辑精选失败")
		return
	}
	s.store.AttachEditorialPickContent(r.Context(), items, userIDFrom(r), true)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// validatePickContent ensures a pick only references a published content item.
// It returns the content for callers that need to show a message.
func (s *Server) validatePickContent(r *http.Request, contentID string) (models.Content, bool) {
	if !uuidPattern.MatchString(contentID) {
		return models.Content{}, false
	}
	content, err := s.store.GetContentAdmin(r.Context(), contentID)
	if err != nil {
		return models.Content{}, false
	}
	return content, content.ContentStatus == "published"
}

func validatePickWindow(in store.EditorialPickInput) bool {
	if in.StartsAt == nil || in.EndsAt == nil {
		return true
	}
	return in.EndsAt.After(*in.StartsAt)
}

func (s *Server) adminCreateEditorialPick(w http.ResponseWriter, r *http.Request) {
	var in store.EditorialPickInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.ContentID = strings.TrimSpace(in.ContentID)
	if in.ContentID == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "请选择关联内容")
		return
	}
	if _, ok := s.validatePickContent(r, in.ContentID); !ok {
		apiError(w, http.StatusBadRequest, "invalid_request", "只能从已发布内容中选择精选")
		return
	}
	if !validatePickWindow(in) {
		apiError(w, http.StatusBadRequest, "invalid_request", "结束时间必须晚于开始时间")
		return
	}
	pick, err := s.store.CreateEditorialPick(r.Context(), in)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "该内容已在精选列表中")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建编辑精选失败")
		return
	}
	items := []models.EditorialPick{pick}
	s.store.AttachEditorialPickContent(r.Context(), items, userIDFrom(r), true)
	writeJSON(w, http.StatusCreated, items[0])
}

func (s *Server) adminUpdateEditorialPick(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !uuidPattern.MatchString(id) {
		apiError(w, http.StatusNotFound, "not_found", "编辑精选不存在")
		return
	}
	existing, err := s.store.GetEditorialPick(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "编辑精选不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取编辑精选失败")
		return
	}
	var in store.EditorialPickInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.ContentID = strings.TrimSpace(in.ContentID)
	if in.ContentID == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "请选择关联内容")
		return
	}
	enabledAfter := existing.Enabled
	if in.Enabled != nil {
		enabledAfter = *in.Enabled
	}
	contentChanged := in.ContentID != existing.ContentID
	if contentChanged || enabledAfter {
		if _, ok := s.validatePickContent(r, in.ContentID); !ok {
			apiError(w, http.StatusBadRequest, "invalid_request", "只能从已发布内容中选择精选")
			return
		}
	}
	if !validatePickWindow(in) {
		apiError(w, http.StatusBadRequest, "invalid_request", "结束时间必须晚于开始时间")
		return
	}
	pick, err := s.store.UpdateEditorialPick(r.Context(), id, in)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "编辑精选不存在")
		return
	}
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "conflict", "该内容已在精选列表中")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "更新编辑精选失败")
		return
	}
	items := []models.EditorialPick{pick}
	s.store.AttachEditorialPickContent(r.Context(), items, userIDFrom(r), true)
	writeJSON(w, http.StatusOK, items[0])
}

func (s *Server) adminDeleteEditorialPick(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !uuidPattern.MatchString(id) {
		apiError(w, http.StatusNotFound, "not_found", "编辑精选不存在")
		return
	}
	if err := s.store.DeleteEditorialPick(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "编辑精选不存在")
		return
	} else if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "删除编辑精选失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
