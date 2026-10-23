package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mrome/internal/media"
	"mrome/internal/util"
)

type uploadMeta struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	RelPath   string `json:"relPath"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"createdAt"`
}

const chunkSize = 4 << 20 // 4 MiB

func (s *Server) uploadMetaPath(id string) string {
	return filepath.Join(s.Cfg.UploadsDir, id+".json")
}

func (s *Server) uploadPartPath(id string) string {
	return filepath.Join(s.Cfg.UploadsDir, id+".part")
}

func (s *Server) loadUpload(id string) (*uploadMeta, error) {
	if v, ok := s.uploads.Load(id); ok {
		return v.(*uploadMeta), nil
	}
	b, err := os.ReadFile(s.uploadMetaPath(id))
	if err != nil {
		return nil, err
	}
	var m uploadMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	s.uploads.Store(id, &m)
	return &m, nil
}

func safeRelPath(rel, filename string) string {
	rel = strings.ReplaceAll(rel, "\\", "/")
	parts := strings.Split(rel, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == ".." {
			continue
		}
		clean = append(clean, p)
	}
	if len(clean) == 0 || clean[len(clean)-1] != filename {
		clean = append(clean, filename)
	}
	return strings.Join(clean, "/")
}

func (s *Server) uploadInit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename     string `json:"filename"`
		RelativePath string `json:"relativePath"`
		Size         int64  `json:"size"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	filename := filepath.Base(strings.TrimSpace(req.Filename))
	if filename == "" || filename == "." {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "文件名无效"})
		return
	}
	if !media.IsAudio(filename) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "仅支持 mp3 / flac / m4a / wav / ogg 等音频格式"})
		return
	}
	if req.Size > s.Cfg.MaxUploadMB<<20 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("文件超过 %d MB 限制", s.Cfg.MaxUploadMB)})
		return
	}
	rel := safeRelPath(req.RelativePath, filename)
	if req.RelativePath == "" {
		rel = "Uploads/" + time.Now().Format("2006-01") + "/" + filename
	}
	id := util.RandomHex(12)
	meta := &uploadMeta{ID: id, Filename: filename, RelPath: rel, Size: req.Size, CreatedAt: time.Now().Unix()}
	if err := os.MkdirAll(s.Cfg.UploadsDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	b, _ := json.Marshal(meta)
	if err := os.WriteFile(s.uploadMetaPath(id), b, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	f, err := os.Create(s.uploadPartPath(id))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	f.Close()
	s.uploads.Store(id, meta)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "chunkSize": chunkSize, "received": 0})
}

func (s *Server) uploadChunk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	meta, err := s.loadUpload(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "上传任务不存在"})
		return
	}
	offset := int64(qInt(r, "offset", -1))
	partPath := s.uploadPartPath(id)

	s.upMu.Lock()
	defer s.upMu.Unlock()

	part, err := os.OpenFile(partPath, os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer part.Close()
	cur, _ := part.Seek(0, io.SeekEnd)
	if offset >= 0 && offset != cur {
		// The client resumes from a different offset: trust the server's file
		// length and ask the client to resync.
		writeJSON(w, http.StatusConflict, map[string]any{"error": "offset 不匹配", "received": cur})
		return
	}
	written, err := io.Copy(part, io.LimitReader(r.Body, chunkSize+(1<<20)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	received := cur + written
	if limit := s.Cfg.MaxUploadMB << 20; received > limit {
		_ = part.Truncate(cur)
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error": fmt.Sprintf("文件超过 %d MB 限制", s.Cfg.MaxUploadMB), "received": cur,
		})
		return
	}
	if meta.Size > 0 && received > meta.Size {
		_ = part.Truncate(meta.Size)
		received = meta.Size
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": received, "chunkSize": chunkSize})
}

func (s *Server) uploadStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	meta, err := s.loadUpload(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "上传任务不存在"})
		return
	}
	var received int64
	if st, err := os.Stat(s.uploadPartPath(id)); err == nil {
		received = st.Size()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "filename": meta.Filename, "size": meta.Size, "received": received, "chunkSize": chunkSize,
	})
}

func (s *Server) uploadComplete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	meta, err := s.loadUpload(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "上传任务不存在"})
		return
	}
	partPath := s.uploadPartPath(id)
	st, err := os.Stat(partPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "尚未收到文件数据"})
		return
	}
	if meta.Size > 0 && st.Size() < meta.Size {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "文件尚未上传完成", "received": st.Size()})
		return
	}

	target := filepath.Join(s.Cfg.MusicDir, filepath.FromSlash(meta.RelPath))
	// Defensive check against path traversal.
	absMusic, _ := filepath.Abs(s.Cfg.MusicDir)
	absTarget, _ := filepath.Abs(target)
	if !strings.HasPrefix(absTarget, absMusic+string(os.PathSeparator)) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "非法的目标路径"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	target = uniquePath(target)
	if err := os.Rename(partPath, target); err != nil {
		// Cross-device fallback.
		if err := copyFile(partPath, target); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		_ = os.Remove(partPath)
	}
	_ = os.Remove(s.uploadMetaPath(id))
	s.uploads.Delete(id)

	songID, scanErr := s.Scanner.ScanFile(target)
	resp := map[string]any{"ok": true, "path": meta.RelPath, "songId": songID}
	if scanErr != nil {
		resp["warning"] = scanErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) uploadAbort(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.uploads.Delete(id)
	_ = os.Remove(s.uploadPartPath(id))
	_ = os.Remove(s.uploadMetaPath(id))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func uniquePath(p string) string {
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
