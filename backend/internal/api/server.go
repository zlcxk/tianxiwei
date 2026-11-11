package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"xiweispace/backend/internal/config"
	"xiweispace/backend/internal/store"
)

type Server struct {
	cfg     config.Config
	store   *store.Store
	limiter *rateLimiter
}

func New(cfg config.Config, st *store.Store) *Server {
	return &Server{cfg: cfg, store: st, limiter: newRateLimiter()}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.securityHeaders)
	r.Use(s.originCheck)
	r.Use(s.requestLogger)
	r.Use(s.loadSession)

	r.Get("/healthz", s.health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.csrfProtect)

		r.Get("/setup/status", s.setupStatus)
		r.Post("/setup", s.setup)
		r.Post("/auth/login", s.login)
		r.Post("/auth/logout", s.logout)
		r.Get("/auth/me", s.me)
		r.Post("/auth/register", s.register)

		r.Get("/artist", s.getArtist)
		r.Get("/home", s.getHome)
		r.Get("/contents", s.listContents)
		r.Get("/contents/facets", s.listContentFacets)
		r.Get("/contents/{id}", s.getContent)
		r.Get("/works", s.listWorks)
		r.Get("/works/{id}", s.getWork)
		r.Get("/characters/{id}", s.getCharacter)
		r.Get("/scenes", s.listScenes)
		r.Get("/events", s.listEvents)
		r.Get("/events/{id}", s.getEvent)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/favorites", s.listFavorites)
			r.Post("/favorites", s.addFavorite)
			r.Delete("/favorites", s.removeFavoriteTarget)
			r.Delete("/favorites/{id}", s.removeFavorite)
			r.Get("/preferences/scenes", s.getPreferences)
			r.Put("/preferences/scenes", s.putPreferences)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(s.requireAdmin)
			r.Get("/overview", s.adminOverview)
			r.Get("/sources", s.adminListSources)
			r.Post("/sources", s.adminCreateSource)
			r.Patch("/sources/{id}", s.adminUpdateSource)
			r.Post("/sources/{id}/run", s.adminRunSource)
			r.Get("/jobs", s.adminListJobs)
			r.Post("/jobs/{id}/retry", s.adminRetryJob)
			r.Get("/reviews", s.adminListReviews)
			r.Post("/reviews/{id}/approve", s.adminApprove)
			r.Post("/reviews/{id}/reject", s.adminReject)
			r.Get("/contents", s.adminListContents)
			r.Post("/contents", s.adminCreateContent)
			r.Get("/contents/{id}", s.adminGetContent)
			r.Patch("/contents/{id}", s.adminUpdateContent)
			r.Get("/artist", s.adminGetArtist)
			r.Put("/artist", s.adminUpdateArtist)
			r.Get("/artist-entity", s.adminGetArtistEntity)
			r.Put("/artist-entity", s.adminPutArtistEntity)
			r.Get("/works", s.adminListWorks)
			r.Post("/works", s.adminCreateWork)
			r.Put("/works/{id}", s.adminUpdateWork)
			r.Delete("/works/{id}", s.adminDeleteWork)
			r.Get("/characters", s.adminListCharacters)
			r.Post("/characters", s.adminCreateCharacter)
			r.Put("/characters/{id}", s.adminUpdateCharacter)
			r.Delete("/characters/{id}", s.adminDeleteCharacter)
			r.Get("/scenes", s.adminListScenes)
			r.Patch("/scenes/{id}", s.adminUpdateScene)
			r.Get("/rules", s.adminListRules)
			r.Post("/rules", s.adminCreateRule)
			r.Put("/rules/{id}", s.adminUpdateRule)
			r.Delete("/rules/{id}", s.adminDeleteRule)
			r.Get("/settings/recommend", s.adminGetRecommendWeights)
			r.Put("/settings/recommend", s.adminPutRecommendWeights)
			r.Get("/search/providers", s.adminListSearchProviders)
			r.Post("/search/providers", s.adminCreateSearchProvider)
			r.Patch("/search/providers/{id}", s.adminUpdateSearchProvider)
			r.Post("/search/providers/{id}/test", s.adminTestSearchProvider)
			r.Get("/search/tasks", s.adminListSearchTasks)
			r.Post("/search/tasks", s.adminCreateSearchTask)
			r.Patch("/search/tasks/{id}", s.adminUpdateSearchTask)
			r.Delete("/search/tasks/{id}", s.adminDeleteSearchTask)
			r.Post("/search/tasks/{id}/run", s.adminRunSearchTask)
			r.Get("/search/runs", s.adminListSearchRuns)
			r.Get("/events", s.adminListEvents)
			r.Post("/events", s.adminCreateEvent)
			r.Put("/events/{id}", s.adminUpdateEvent)
			r.Delete("/events/{id}", s.adminDeleteEvent)
		})
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Pool.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", r.RemoteAddr,
		)
	})
}
