package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"xiweispace/backend/internal/store"
)

func (s *Server) listFavorites(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	targetType := r.URL.Query().Get("type")
	if targetType != "" && !isOneOf(targetType, "content", "work", "character") {
		apiError(w, http.StatusBadRequest, "invalid_request", "收藏类型不正确")
		return
	}
	items, err := s.store.ListFavorites(r.Context(), u.ID, targetType)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取收藏失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) addFavorite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	in.TargetType = strings.TrimSpace(in.TargetType)
	in.TargetID = strings.TrimSpace(in.TargetID)
	if !isOneOf(in.TargetType, "content", "work", "character") || in.TargetID == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "收藏对象不正确")
		return
	}
	u := userFrom(r.Context())
	id, err := s.store.AddFavorite(r.Context(), u.ID, in.TargetType, in.TargetID)
	if err == store.ErrNotFound {
		apiError(w, http.StatusNotFound, "not_found", "要收藏的对象不存在或已下架")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "收藏失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (s *Server) removeFavorite(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if err := s.store.RemoveFavorite(r.Context(), u.ID, chi.URLParam(r, "id")); err != nil {
		apiError(w, http.StatusNotFound, "not_found", "收藏不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) removeFavoriteTarget(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	targetType := r.URL.Query().Get("target_type")
	targetID := r.URL.Query().Get("target_id")
	if !isOneOf(targetType, "content", "work", "character") || targetID == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "收藏对象不正确")
		return
	}
	if err := s.store.RemoveFavoriteTarget(r.Context(), u.ID, targetType, targetID); err != nil {
		apiError(w, http.StatusNotFound, "not_found", "收藏不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) getPreferences(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	scenes, err := s.store.ListScenes(r.Context(), u.ID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取兴趣设置失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": scenes})
}

func (s *Server) putPreferences(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Preferences []store.ScenePrefInput `json:"preferences"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Preferences) > 20 {
		apiError(w, http.StatusBadRequest, "invalid_request", "兴趣设置数量过多")
		return
	}
	u := userFrom(r.Context())
	if err := s.store.SetScenePreferences(r.Context(), u.ID, in.Preferences); err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "保存兴趣设置失败")
		return
	}
	scenes, err := s.store.ListScenes(r.Context(), u.ID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取兴趣设置失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": scenes})
}
