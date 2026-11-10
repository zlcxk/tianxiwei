package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

func (s *Server) getArtist(w http.ResponseWriter, r *http.Request) {
	artist, err := s.store.GetArtist(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "艺人资料尚未建立")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取艺人资料失败")
		return
	}
	writeJSON(w, http.StatusOK, artist)
}

func (s *Server) getHome(w http.ResponseWriter, r *http.Request) {
	userID := ""
	if u := userFrom(r.Context()); u != nil {
		userID = u.ID
	}
	data, err := s.store.Home(r.Context(), userID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取首页失败")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) listContents(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := pagination(r)
	q := r.URL.Query()
	var scenes []string
	for _, sc := range strings.Split(q.Get("scene"), ",") {
		if v := strings.TrimSpace(sc); v != "" {
			scenes = append(scenes, v)
		}
	}
	filter := store.ContentFilter{
		Keyword:     strings.TrimSpace(q.Get("keyword")),
		Scenes:      scenes,
		SourceType:  strings.TrimSpace(q.Get("source_type")),
		SourceQuery: strings.TrimSpace(q.Get("source")),
		ContentType: strings.TrimSpace(q.Get("content_type")),
		Official:    parseBoolQuery(r, "official"),
		WorkID:      strings.TrimSpace(q.Get("work_id")),
		CharacterID: strings.TrimSpace(q.Get("character_id")),
		StartTime:   timeRangeStart(r),
		Sort:        strings.TrimSpace(q.Get("sort")),
		Limit:       pageSize,
		Offset:      offset,
	}
	userID := ""
	if u := userFrom(r.Context()); u != nil {
		userID = u.ID
	}
	var items []models.Content
	var total int
	var err error
	if filter.Sort == "recommended" {
		items, total, err = s.store.ListRecommended(r.Context(), filter, userID)
	} else {
		items, total, err = s.store.ListContents(r.Context(), filter)
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取内容失败")
		return
	}
	s.store.DecorateContentFavorites(r.Context(), userID, items)
	writeJSON(w, http.StatusOK, models.Paged[models.Content]{
		Items: items, Page: page, PageSize: pageSize, Total: total,
	})
}

func (s *Server) getContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := s.store.GetContent(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取内容失败")
		return
	}
	user := userFrom(r.Context())
	if c.ContentStatus != "published" && (user == nil || (user.Role != "admin" && user.Role != "editor")) {
		apiError(w, http.StatusNotFound, "not_found", "内容不存在")
		return
	}
	if user != nil {
		c.Favorited = s.store.IsFavorited(r.Context(), user.ID, "content", c.ID)
	}
	c.RawData = nil
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) listWorks(w http.ResponseWriter, r *http.Request) {
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
	if u := userFrom(r.Context()); u != nil {
		for i := range works {
			works[i].Favorited = s.store.IsFavorited(r.Context(), u.ID, "work", works[i].ID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": works})
}

func (s *Server) getWork(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	work, err := s.store.GetWork(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "作品不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取作品失败")
		return
	}
	userID := ""
	if u := userFrom(r.Context()); u != nil {
		userID = u.ID
		work.Favorited = s.store.IsFavorited(r.Context(), userID, "work", work.ID)
	}
	contents, err := s.store.ContentsForWork(r.Context(), work.ID, userID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取关联内容失败")
		return
	}
	writeJSON(w, http.StatusOK, models.WorkDetail{Work: work, Contents: contents})
}

func (s *Server) getCharacter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	character, err := s.store.GetCharacter(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not_found", "角色不存在")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取角色失败")
		return
	}
	userID := ""
	if u := userFrom(r.Context()); u != nil {
		userID = u.ID
		character.Favorited = s.store.IsFavorited(r.Context(), userID, "character", character.ID)
	}
	contents, err := s.store.ContentsForCharacter(r.Context(), character.ID, userID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取关联内容失败")
		return
	}
	writeJSON(w, http.StatusOK, models.CharacterDetail{Character: character, Contents: contents})
}

func (s *Server) listScenes(w http.ResponseWriter, r *http.Request) {
	userID := ""
	if u := userFrom(r.Context()); u != nil {
		userID = u.ID
	}
	scenes, err := s.store.ListScenes(r.Context(), userID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "读取场景失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": scenes})
}
