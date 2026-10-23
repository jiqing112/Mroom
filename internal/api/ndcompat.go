package api

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"mrome/internal/util"
)

// Navidrome compatibility layer.
//
// Some clients (音流 / MusicFlow among them) offer a "Navidrome" server type
// that does not use the Subsonic u/p/t/s handshake at all: it POSTs
// /auth/login, expects a JWT back, and then sends "X-ND-Authorization: Bearer".
// Navidrome also hands out a pre-computed Subsonic salt/token pair so the client
// can call /rest/* with the classic scheme, which is what most of these clients
// do for streaming. Serving these few endpoints is enough to make that mode work.

type ndLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) ndLogin(w http.ResponseWriter, r *http.Request) {
	key := util.ClientKey(r)
	if s.loginGuard.Blocked(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "尝试次数过多，请 5 分钟后再试"})
		return
	}
	var req ndLoginReq
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	u, err := s.Store.Authenticate(strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		s.loginGuard.Fail(key)
		time.Sleep(350 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "用户名或密码错误"})
		return
	}
	s.loginGuard.Reset(key)
	salt := util.RandomHex(8)
	sum := md5.Sum([]byte(req.Password + salt))
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                u.ID,
		"username":          u.Username,
		"name":              u.Username,
		"isAdmin":           u.IsAdmin,
		"token":             util.SignJWT(s.secret, u.ID, 30*24*time.Hour),
		"subsonicSalt":      salt,
		"subsonicToken":     hex.EncodeToString(sum[:]),
		"avatarUrl":         "",
		"lastFMApiKey":      "",
		"lastFMSession":     "",
		"listenBrainzToken": "",
	})
}

func (s *Server) ndLogout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ndKeepalive keeps clients that poll it from logging connection errors.
func (s *Server) ndKeepalive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// ndPlayQueue answers Navidrome's server-side play queue. mrome keeps the queue
// in the browser, so reads report an empty queue and writes are accepted and
// ignored — enough for clients that poll it without treating it as an error.
func (s *Server) ndPlayQueue(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.ndCurrentUser(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "未登录"})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"id":        "",
			"current":   0,
			"position":  0,
			"changed":   "",
			"changedBy": "",
			"entries":   []any{},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// ndScrobble records a finished play so client-side listening stats land in the
// same annotations table the web player uses.
func (s *Server) ndScrobble(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.ndCurrentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "未登录"})
		return
	}
	var req struct {
		ID         string `json:"id"`
		Submission bool   `json:"submission"`
	}
	_ = readJSON(r, &req)
	if req.Submission && req.ID != "" {
		if err := s.Store.Scrobble(uid, req.ID, time.Now().Unix()); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// ndCurrentUser resolves the JWT that a Navidrome-mode client presents.
func (s *Server) ndCurrentUser(r *http.Request) (string, bool) {
	tok := util.BearerToken(r.Header.Get)
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("jwt"))
	}
	if tok == "" {
		return "", false
	}
	return util.ParseJWT(tok, s.secret)
}
