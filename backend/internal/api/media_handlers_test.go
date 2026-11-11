package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"xiweispace/backend/internal/config"
	"xiweispace/backend/internal/models"
)

var (
	pngSignature  = []byte("\x89PNG\r\n\x1a\n")
	jpegSignature = []byte("\xff\xd8\xff\xe0")
	webpSignature = []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{cfg: config.Config{MediaDir: t.TempDir()}, limiter: newRateLimiter()}
}

func multipartImage(t *testing.T, name string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func uploadRequest(t *testing.T, body *bytes.Buffer, contentType, role string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/media/images", body)
	req.Header.Set("Content-Type", contentType)
	if role != "" {
		sess := &models.AuthSession{User: models.User{Role: role}, CSRFToken: "token"}
		req = req.WithContext(context.WithValue(req.Context(), sessionKey, sess))
	}
	return req
}

func TestMediaUploadRequiresAdminOrEditor(t *testing.T) {
	s := newTestServer(t)
	handler := s.requireAdmin(http.HandlerFunc(s.adminUploadImage))

	for _, role := range []string{"", "viewer"} {
		body, contentType := multipartImage(t, "cover.png", append(pngSignature, []byte("payload")...))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, uploadRequest(t, body, contentType, role))
		want := http.StatusUnauthorized
		if role == "viewer" {
			want = http.StatusForbidden
		}
		if rec.Code != want {
			t.Fatalf("role %q upload status = %d, want %d", role, rec.Code, want)
		}
	}
}

func TestMediaUploadAcceptsSupportedTypesAndServesPublicly(t *testing.T) {
	cases := []struct {
		name     string
		original string
		content  []byte
		wantType string
	}{
		{"png", "../../manual name.PNG", append(pngSignature, []byte("png-body")...), "image/png"},
		{"jpeg", "cover.jpg", append(jpegSignature, []byte("jpeg-body")...), "image/jpeg"},
		{"webp", "cover.webp", append(webpSignature, []byte("webp-body")...), "image/webp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestServer(t)
			body, contentType := multipartImage(t, tc.original, tc.content)
			rec := httptest.NewRecorder()
			s.adminUploadImage(rec, uploadRequest(t, body, contentType, "editor"))
			if rec.Code != http.StatusCreated {
				t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var payload struct {
				URL         string `json:"url"`
				Filename    string `json:"filename"`
				ContentType string `json:"content_type"`
				Size        int64  `json:"size"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode upload response: %v", err)
			}
			if !mediaNamePattern.MatchString(payload.Filename) {
				t.Fatalf("filename %q is not a controlled random name", payload.Filename)
			}
			if strings.Contains(payload.Filename, "manual") || strings.Contains(payload.Filename, "..") {
				t.Fatalf("client filename leaked into stored name: %q", payload.Filename)
			}
			if payload.ContentType != tc.wantType {
				t.Fatalf("content type = %q, want %q", payload.ContentType, tc.wantType)
			}
			if payload.URL != "/api/v1/media/"+payload.Filename {
				t.Fatalf("url = %q", payload.URL)
			}
			if payload.Size != int64(len(tc.content)) {
				t.Fatalf("size = %d, want %d", payload.Size, len(tc.content))
			}
			if _, err := os.Stat(filepath.Join(s.cfg.MediaDir, payload.Filename)); err != nil {
				t.Fatalf("uploaded file missing: %v", err)
			}

			router := chi.NewRouter()
			router.Get("/api/v1/media/{filename}", s.publicMedia)
			serve := httptest.NewRecorder()
			router.ServeHTTP(serve, httptest.NewRequest(http.MethodGet, payload.URL, nil))
			if serve.Code != http.StatusOK {
				t.Fatalf("public media status = %d", serve.Code)
			}
			if got := serve.Header().Get("Content-Type"); got != tc.wantType {
				t.Fatalf("served content type = %q, want %q", got, tc.wantType)
			}
			if got := serve.Header().Get("Cache-Control"); !strings.Contains(got, "public") {
				t.Fatalf("public media cache-control = %q", got)
			}
			if !bytes.Equal(serve.Body.Bytes(), tc.content) {
				t.Fatal("served bytes differ from uploaded bytes")
			}
		})
	}
}

func TestMediaUploadRejectsUnsupportedAndOversizedFiles(t *testing.T) {
	s := newTestServer(t)

	body, contentType := multipartImage(t, "notes.txt", []byte("just text, not an image"))
	rec := httptest.NewRecorder()
	s.adminUploadImage(rec, uploadRequest(t, body, contentType, "admin"))
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text upload status = %d, want 415", rec.Code)
	}

	oversized := append(append([]byte{}, pngSignature...), bytes.Repeat([]byte{0}, maxImageBytes+1)...)
	body, contentType = multipartImage(t, "huge.png", oversized)
	rec = httptest.NewRecorder()
	s.adminUploadImage(rec, uploadRequest(t, body, contentType, "admin"))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload status = %d, want 413", rec.Code)
	}

	entries, err := os.ReadDir(s.cfg.MediaDir)
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected uploads left files behind: %d", len(entries))
	}
}

func TestMediaFilenameValidationBlocksTraversal(t *testing.T) {
	valid := []string{
		"0123456789abcdef0123456789abcdef.jpg",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png",
		"ffffffffffffffffffffffffffffffff.webp",
	}
	for _, name := range valid {
		if !mediaNamePattern.MatchString(name) {
			t.Fatalf("valid name %q rejected", name)
		}
	}
	invalid := []string{
		"../evil.png",
		"..%2Fevil.png",
		"sub/dir.png",
		"0123456789abcdef0123456789abcdef.gif",
		"0123456789abcdef0123456789abcdef.JPG",
		"0123456789abcdef0123456789abcde.jpg",
		".upload-123456",
		"",
	}
	for _, name := range invalid {
		if mediaNamePattern.MatchString(name) {
			t.Fatalf("unsafe name %q accepted", name)
		}
	}

	s := newTestServer(t)
	router := chi.NewRouter()
	router.Get("/api/v1/media/{filename}", s.publicMedia)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/media/..%2Fsecret.png", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("traversal request status = %d, want 404", rec.Code)
	}
}
