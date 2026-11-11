package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	maxImageBytes = 10 << 20
	// multipart bodies carry boundary and part headers on top of the file.
	maxImageRequestBytes = maxImageBytes + (1 << 20)
)

// mediaNamePattern is deliberately narrow: uploads always use a random
// 32-hex-character UUIDv4 name plus an extension derived from the sniffed
// content type. Nothing supplied by the client reaches the filesystem path.
var mediaNamePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|webp)$`)

var imageExtensionByType = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

func mediaContentType(name string) string {
	switch strings.TrimPrefix(filepath.Ext(name), ".") {
	case "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// newMediaFilename returns a UUIDv4-style random filename with an extension
// chosen from the sniffed MIME type, never from the uploaded filename.
func newMediaFilename(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b) + "." + ext, nil
}

// adminUploadImage accepts a single multipart field named "file". It is only
// reachable through the admin/editor middleware. The file type is derived from
// the bytes themselves, the size is capped at 10MB, and the write is atomic so
// a failed upload never leaves a partial image behind.
func (s *Server) adminUploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageRequestBytes)
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			apiError(w, http.StatusRequestEntityTooLarge, "image_too_large", "图片不能超过 10MB")
			return
		}
		apiError(w, http.StatusBadRequest, "invalid_request", "请选择要上传的图片")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", "请选择要上传的图片")
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		apiError(w, http.StatusBadRequest, "invalid_request", "图片读取失败")
		return
	}
	ext, ok := imageExtensionByType[http.DetectContentType(head[:n])]
	if !ok {
		apiError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "仅支持 JPEG、PNG 或 WebP 图片")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		apiError(w, http.StatusBadRequest, "invalid_request", "图片读取失败")
		return
	}

	if err := os.MkdirAll(s.cfg.MediaDir, 0o755); err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "图片存储目录不可用")
		return
	}
	name, err := newMediaFilename(ext)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "图片命名失败")
		return
	}
	tmp, err := os.CreateTemp(s.cfg.MediaDir, ".upload-*")
	if err != nil {
		apiError(w, http.StatusInternalServerError, "internal", "图片写入失败")
		return
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}
	written, err := io.Copy(tmp, io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		cleanup()
		apiError(w, http.StatusInternalServerError, "internal", "图片写入失败")
		return
	}
	if written > maxImageBytes {
		cleanup()
		apiError(w, http.StatusRequestEntityTooLarge, "image_too_large", "图片不能超过 10MB")
		return
	}
	if err := tmp.Chmod(0o644); err != nil {
		cleanup()
		apiError(w, http.StatusInternalServerError, "internal", "图片写入失败")
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		apiError(w, http.StatusInternalServerError, "internal", "图片写入失败")
		return
	}
	finalPath := filepath.Join(s.cfg.MediaDir, name)
	if err := os.Rename(tmpName, finalPath); err != nil {
		os.Remove(tmpName)
		apiError(w, http.StatusInternalServerError, "internal", "图片保存失败")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"url":          "/api/v1/media/" + name,
		"filename":     name,
		"content_type": mediaContentType(name),
		"size":         written,
	})
}

// publicMedia serves uploaded images without a login. Names are immutable and
// validated against a strict pattern, so responses can be cached for a year.
// The global no-store header is overridden here only.
func (s *Server) publicMedia(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if !mediaNamePattern.MatchString(name) {
		apiError(w, http.StatusNotFound, "not_found", "图片不存在")
		return
	}
	f, err := os.Open(filepath.Join(s.cfg.MediaDir, name))
	if err != nil {
		apiError(w, http.StatusNotFound, "not_found", "图片不存在")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		apiError(w, http.StatusNotFound, "not_found", "图片不存在")
		return
	}
	w.Header().Set("Content-Type", mediaContentType(name))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, name, info.ModTime(), f)
}
