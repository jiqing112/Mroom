package util

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MD5Hex builds a stable, opaque, string-typed identifier (Subsonic requires strings).
func MD5Hex(parts ...string) string {
	h := md5.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0x1f})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

func QueryInt(r *http.Request, key string, def int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// WriteJSON encodes v as a JSON response body.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// ServeMedia streams an audio file with Range / HEAD semantics. attachment adds
// a Content-Disposition attachment header (Subsonic download).
func ServeMedia(w http.ResponseWriter, r *http.Request, path, contentType string, attachment bool) error {
	f, st, err := openStat(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache")
	if attachment {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(path)))
	}
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), f)
	return nil
}

// ServeImage streams artwork with long-lived cache headers. etag is a prefix
// combined with the file's modification time so changed art busts the cache.
func ServeImage(w http.ResponseWriter, r *http.Request, path, contentType, etag string) error {
	f, st, err := openStat(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if contentType == "" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	if etag != "" {
		w.Header().Set("ETag", `"`+etag+`-`+strconv.FormatInt(st.ModTime().Unix(), 10)+`"`)
	}
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), f)
	return nil
}

func openStat(path string) (*os.File, os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, st, nil
}

// CleanName normalises whitespace in metadata strings.
func CleanName(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func Clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// ClientKey identifies the caller for rate limiting: the first hop in
// X-Forwarded-For when a reverse proxy is in front, otherwise the peer address.
func ClientKey(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.Index(v, ","); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

// MD5File computes the hex MD5 of the file at path, streaming so large audio
// files never sit in memory.
func MD5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 1<<20)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// UniquePath returns p, or p (2), p (3)... for the first name that does not
// exist yet, so a new file never silently overwrites an old one.
func UniquePath(p string) string {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return p
}
