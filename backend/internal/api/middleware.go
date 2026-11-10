package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"xiweispace/backend/internal/auth"
	"xiweispace/backend/internal/models"
)

const cookieName = "xiwei_session"

type ctxKey int

const sessionKey ctxKey = 1

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.cfg.SessionTTL.Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) loadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err == nil && cookie.Value != "" {
			sess, err := s.store.GetSession(r.Context(), auth.TokenHash(cookie.Value))
			if err == nil {
				ctx := context.WithValue(r.Context(), sessionKey, &sess)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			s.clearSessionCookie(w)
		}
		next.ServeHTTP(w, r)
	})
}

func sessionFrom(ctx context.Context) *models.AuthSession {
	sess, _ := ctx.Value(sessionKey).(*models.AuthSession)
	return sess
}

func userFrom(ctx context.Context) *models.User {
	if sess := sessionFrom(ctx); sess != nil {
		return &sess.User
	}
	return nil
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sessionFrom(r.Context()) == nil {
			apiError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := sessionFrom(r.Context())
		if sess == nil {
			apiError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		if sess.User.Role != "admin" && sess.User.Role != "editor" {
			apiError(w, http.StatusForbidden, "forbidden", "没有管理权限")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/setup" || r.URL.Path == "/api/v1/auth/register" {
			next.ServeHTTP(w, r)
			return
		}
		sess := sessionFrom(r.Context())
		if sess == nil {
			next.ServeHTTP(w, r)
			return
		}
		header := r.Header.Get("X-CSRF-Token")
		if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(sess.CSRFToken)) != 1 {
			apiError(w, http.StatusForbidden, "csrf_failed", "请求校验失败，请刷新页面后重试")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && !s.allowedOrigin(r, origin) {
			apiError(w, http.StatusForbidden, "origin_rejected", "请求来源不被允许")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) allowedOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	origin = strings.TrimRight(origin, "/")
	for _, allowed := range s.cfg.AllowedOrigins {
		if strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}
