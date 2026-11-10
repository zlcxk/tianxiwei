package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"xiweispace/backend/internal/auth"
	"xiweispace/backend/internal/models"
	"xiweispace/backend/internal/store"
)

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.CountUsers(r.Context())
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "无法读取初始化状态")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"needs_setup":        count == 0,
		"allow_registration": s.cfg.AllowRegistration,
	})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateCredentials(in.Email, in.Password); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建账户失败")
		return
	}
	nickname := strings.TrimSpace(in.Nickname)
	if nickname == "" {
		nickname = "管理员"
	}
	user, err := s.store.SetupAdmin(r.Context(), in.Email, hash, nickname)
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "already_initialized", "站点已经完成初始化")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建管理员失败")
		return
	}
	csrf, err := s.startSession(w, r, user)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "登录会话创建失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "csrf_token": csrf})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.limiter.allow("login:"+ip, 10, 10*time.Minute) {
		apiError(w, http.StatusTooManyRequests, "rate_limited", "尝试次数过多，请稍后再试")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Email == "" || in.Password == "" {
		apiError(w, http.StatusBadRequest, "invalid_request", "请输入邮箱和密码")
		return
	}
	cred, err := s.store.GetCredentialByEmail(r.Context(), in.Email)
	if err != nil || !auth.VerifyPassword(cred.PasswordHash, in.Password) {
		apiError(w, http.StatusUnauthorized, "invalid_credentials", "邮箱或密码不正确")
		return
	}
	if cred.User.Status != "active" {
		apiError(w, http.StatusForbidden, "account_disabled", "账户已被停用")
		return
	}
	s.limiter.reset("login:" + ip)
	csrf, err := s.startSession(w, r, cred.User)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "登录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": cred.User, "csrf_token": csrf})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AllowRegistration {
		apiError(w, http.StatusForbidden, "registration_closed", "当前未开放注册")
		return
	}
	ip := clientIP(r)
	if !s.limiter.allow("register:"+ip, 5, time.Hour) {
		apiError(w, http.StatusTooManyRequests, "rate_limited", "注册过于频繁，请稍后再试")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateCredentials(in.Email, in.Password); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建账户失败")
		return
	}
	nickname := strings.TrimSpace(in.Nickname)
	if nickname == "" {
		nickname = strings.Split(in.Email, "@")[0]
	}
	user, err := s.store.CreateUser(r.Context(), in.Email, hash, nickname, "user")
	if errors.Is(err, store.ErrConflict) {
		apiError(w, http.StatusConflict, "email_exists", "该邮箱已注册")
		return
	}
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "创建账户失败")
		return
	}
	csrf, err := s.startSession(w, r, user)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "登录会话创建失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "csrf_token": csrf})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		_ = s.store.DeleteSession(r.Context(), auth.TokenHash(cookie.Value))
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if sess == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"user":               nil,
			"csrf_token":         "",
			"allow_registration": s.cfg.AllowRegistration,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":               sess.User,
		"csrf_token":         sess.CSRFToken,
		"allow_registration": s.cfg.AllowRegistration,
	})
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user models.User) (string, error) {
	token, err := auth.NewSessionToken()
	if err != nil {
		return "", err
	}
	csrf, err := auth.NewCSRFToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(s.cfg.SessionTTL)
	if err := s.store.CreateSession(r.Context(), user.ID, auth.TokenHash(token), csrf,
		r.UserAgent(), clientIP(r), expires); err != nil {
		return "", err
	}
	s.setSessionCookie(w, token)
	return csrf, nil
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	return strings.Trim(ip, "[]")
}
