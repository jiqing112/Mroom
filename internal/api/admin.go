package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mrome/internal/store"
)

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	stats, _ := s.Store.Stats()
	users, _ := s.Store.ListUsers()
	admins, _ := s.Store.CountAdmins()
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	base := scheme + "://" + r.Host
	writeJSON(w, http.StatusOK, map[string]any{
		"musicDir":    s.Cfg.MusicDir,
		"dataDir":     s.Cfg.DataDir,
		"dbPath":      s.Cfg.DBPath,
		"maxUploadMB": s.Cfg.MaxUploadMB,
		"stats":       stats,
		"users":       users,
		"scan":        s.Scanner.Status(),
		"subsonicUrl": base + "/rest",
		"version":     "1.0.0",
		"apiVersion":  "1.16.1",
		"guestMode":   s.guestEnabled(),
		"canLogin":    admins > 0,
		"plexToken":   s.plexToken,
	})
}

// setGuestModeHandler is the one-click switch behind Settings → Users. Turning it
// on makes every anonymous visitor a guest; turning it off requires at least one
// administrator, otherwise nobody could ever sign back in.
func (s *Server) setGuestModeHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if req.Enabled {
		if _, err := s.ensureGuest(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "无法创建访客账号：" + err.Error()})
			return
		}
	} else if n, _ := s.Store.CountAdmins(); n == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "还没有管理员账号。请先创建一个管理员，再关闭免登录，否则将无法登录。",
		})
		return
	}
	s.setGuestMode(req.Enabled)
	log.Printf("免登录访问已%s", map[bool]string{true: "开启", false: "关闭"}[req.Enabled])
	writeJSON(w, http.StatusOK, map[string]any{"guestMode": s.guestEnabled()})
}

func (s *Server) scanStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"scan": s.Scanner.Status()})
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	if !s.Scanner.Start(context.Background()) {
		writeJSON(w, http.StatusOK, map[string]any{"scan": s.Scanner.Status(), "already": true})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"scan": s.Scanner.Status()})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"isAdmin"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	u, err := s.Store.CreateUser(req.Username, req.Password, req.IsAdmin)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Password string `json:"password"`
		IsAdmin  *bool  `json:"isAdmin"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if req.Password != "" && len(req.Password) < 3 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "密码至少 3 位"})
		return
	}
	if err := s.Store.UpdateUser(id, req.Password, req.IsAdmin); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteUser(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) regenAPIKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.Store.RegenerateAPIKey(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apiKey": key})
}

func (s *Server) spa() http.Handler {
	index := filepath.Join(s.Cfg.WebDir, "index.html")
	fs := http.FileServer(http.Dir(s.Cfg.WebDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean("/" + r.URL.Path)
		full := filepath.Join(s.Cfg.WebDir, filepath.FromSlash(clean))
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fs.ServeHTTP(w, r)
			return
		}
		if _, err := os.Stat(index); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("前端尚未构建。请在 web/ 目录执行 npm install && npm run build，或用 docker compose 一键启动。"))
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}

var _ = store.User{}
