package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func apiError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			apiError(w, http.StatusBadRequest, "invalid_request", "请求内容不能为空")
			return false
		}
		apiError(w, http.StatusBadRequest, "invalid_request", "请求格式不正确")
		return false
	}
	return true
}

func validEmail(s string) bool {
	addr, err := mail.ParseAddress(strings.TrimSpace(s))
	return err == nil && strings.Contains(addr.Address, "@") && len(addr.Address) <= 254
}

func validateCredentials(email, password string) error {
	if !validEmail(email) {
		return fmt.Errorf("请输入有效的邮箱地址")
	}
	if len([]rune(password)) < 10 {
		return fmt.Errorf("密码至少需要 10 个字符")
	}
	if len(password) > 128 {
		return fmt.Errorf("密码过长")
	}
	return nil
}

func pagination(r *http.Request) (page, pageSize, offset int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 50 {
		pageSize = 12
	}
	offset = (page - 1) * pageSize
	return
}

func parseBoolQuery(r *http.Request, key string) *bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &v
}

func timeRangeStart(r *http.Request) *time.Time {
	now := time.Now()
	var t time.Time
	switch r.URL.Query().Get("range") {
	case "today":
		y, m, d := now.Date()
		t = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case "7d":
		t = now.AddDate(0, 0, -7)
	case "30d":
		t = now.AddDate(0, 0, -30)
	case "90d":
		t = now.AddDate(0, 0, -90)
	default:
		return nil
	}
	return &t
}

func parseRFC3339(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339, raw)
	return t
}

func isOneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}
