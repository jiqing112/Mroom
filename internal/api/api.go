package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"mrome/internal/config"
	"mrome/internal/ratelimit"
	"mrome/internal/scanner"
	"mrome/internal/store"
	"mrome/internal/util"
)

type Server struct {
	Store   *store.Store
	Scanner *scanner.Scanner
	Cfg     config.Config
	secret  []byte
	uploads sync.Map
	upMu    sync.Mutex
	// Guest mode is switchable at runtime from the settings panel, so it lives
	// here instead of being read from Cfg on every request. guestID caches the
	// shared guest account id to keep the auth middleware off the settings table.
	guestMu   sync.RWMutex
	guestMode bool
	guestID   string
	// plexToken is the fixed token Plex clients must present; it is derived from
	// the instance secret by the plex package and surfaced in the settings UI.
	plexToken string
	// loginGuard throttles password guessing per client IP.
	loginGuard *ratelimit.Guard
}

// SetPlexToken records the token shown to users for the Plex-compatible endpoints.
func (s *Server) SetPlexToken(t string) { s.plexToken = t }

func New(st *store.Store, sc *scanner.Scanner, cfg config.Config) *Server {
	s := &Server{
		Store: st, Scanner: sc, Cfg: cfg,
		secret:     []byte(st.GetSetting("secret", "mrome")),
		loginGuard: ratelimit.New(8, 5*time.Minute),
	}
	s.guestMode = cfg.GuestMode
	s.guestID = st.GetSetting("guest_user_id", "")
	return s
}

// guestEnabled reports whether visitors may browse without signing in.
func (s *Server) guestEnabled() bool {
	s.guestMu.RLock()
	defer s.guestMu.RUnlock()
	return s.guestMode
}

// setGuestMode flips guest mode and persists the choice so it survives restarts
// and keeps precedence over the config file.
func (s *Server) setGuestMode(on bool) {
	s.guestMu.Lock()
	s.guestMode = on
	s.guestMu.Unlock()
	v := "0"
	if on {
		v = "1"
	}
	_ = s.Store.SetSetting("guest_mode", v)
}

// ensureGuest creates (or reuses) the shared guest account and remembers its id.
func (s *Server) ensureGuest() (*store.User, error) {
	g, err := s.Store.EnsureGuest()
	if err == nil && g != nil {
		s.guestMu.Lock()
		s.guestID = g.ID
		s.guestMu.Unlock()
	}
	return g, err
}

type ctxKey string

const userKey ctxKey = "user"

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.Handle("GET /api/auth/me", s.auth(s.me))
	mux.HandleFunc("GET /api/meta", s.meta)

	// Navidrome-compatible handshake, used by clients that pick that server type.
	mux.HandleFunc("POST /auth/login", s.ndLogin)
	mux.HandleFunc("POST /auth/logout", s.ndLogout)
	mux.HandleFunc("/api/keepalive", s.ndKeepalive)
	mux.HandleFunc("/api/playqueue", s.ndPlayQueue)
	mux.HandleFunc("/api/playqueue/", s.ndPlayQueue)
	mux.HandleFunc("/api/scrobble", s.ndScrobble)

	mux.Handle("GET /api/home", s.auth(s.home))
	mux.Handle("GET /api/songs", s.auth(s.listSongs))
	mux.Handle("GET /api/songs/{id}", s.auth(s.getSong))
	mux.Handle("PATCH /api/songs/{id}", s.admin(s.updateSong))
	mux.Handle("DELETE /api/songs/{id}", s.admin(s.deleteSong))
	mux.Handle("GET /api/songs/{id}/lyrics", s.auth(s.lyrics))
	mux.Handle("GET /api/artists", s.auth(s.listArtists))
	mux.Handle("GET /api/artists/{id}", s.auth(s.getArtist))
	mux.Handle("GET /api/albums", s.auth(s.listAlbums))
	mux.Handle("GET /api/albums/{id}", s.auth(s.getAlbum))
	mux.Handle("GET /api/genres", s.auth(s.listGenres))
	mux.Handle("GET /api/search", s.auth(s.search))
	mux.Handle("GET /api/stars", s.auth(s.stars))
	mux.Handle("GET /api/recent", s.auth(s.recentSongs))

	mux.Handle("GET /api/playlists", s.auth(s.listPlaylists))
	mux.Handle("POST /api/playlists", s.auth(s.createPlaylist))
	mux.Handle("GET /api/playlists/{id}", s.auth(s.getPlaylist))
	mux.Handle("PATCH /api/playlists/{id}", s.auth(s.updatePlaylist))
	mux.Handle("DELETE /api/playlists/{id}", s.auth(s.deletePlaylist))
	mux.Handle("POST /api/playlists/{id}/songs", s.auth(s.addPlaylistSongs))
	mux.Handle("DELETE /api/playlists/{id}/songs/{songId}", s.auth(s.removePlaylistSong))

	mux.Handle("POST /api/star", s.auth(s.setStar))
	mux.Handle("POST /api/rating", s.auth(s.setRating))
	mux.Handle("POST /api/scrobble", s.auth(s.scrobble))

	mux.Handle("GET /api/stream/{id}", s.auth(s.stream))
	mux.Handle("GET /api/cover/{id}", s.auth(s.cover))
	mux.Handle("GET /api/cover/{id}/{size}", s.auth(s.cover))

	mux.Handle("POST /api/upload/init", s.admin(s.uploadInit))
	mux.Handle("PUT /api/upload/{id}", s.admin(s.uploadChunk))
	mux.Handle("GET /api/upload/{id}", s.admin(s.uploadStatus))
	mux.Handle("POST /api/upload/{id}/complete", s.admin(s.uploadComplete))
	mux.Handle("DELETE /api/upload/{id}", s.admin(s.uploadAbort))

	mux.Handle("GET /api/admin/settings", s.admin(s.adminSettings))
	mux.Handle("PATCH /api/admin/guest-mode", s.admin(s.setGuestModeHandler))
	mux.Handle("GET /api/admin/scan", s.admin(s.scanStatus))
	mux.Handle("POST /api/admin/scan", s.admin(s.startScan))
	mux.Handle("GET /api/admin/users", s.admin(s.listUsers))
	mux.Handle("POST /api/admin/users", s.admin(s.createUser))
	mux.Handle("PATCH /api/admin/users/{id}", s.admin(s.updateUser))
	mux.Handle("DELETE /api/admin/users/{id}", s.admin(s.deleteUser))
	mux.Handle("POST /api/admin/users/{id}/apikey", s.admin(s.regenAPIKey))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "接口不存在"})
	})

	mux.Handle("/", s.spa())
	return s.common(mux)
}

func (s *Server) common(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "服务器内部错误"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------- auth

func (s *Server) sign(payload string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) newSession(u *store.User) string {
	exp := time.Now().Add(30 * 24 * time.Hour).Unix()
	payload := u.ID + "|" + strconv.FormatInt(exp, 10) + "|" + strconv.FormatInt(u.SessionVersion, 10)
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + s.sign(payload)
	return token
}

func (s *Server) readSession(r *http.Request) (*store.User, error) {
	c, err := r.Cookie("mrome_session")
	if err != nil {
		return nil, err
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return nil, errors.New("bad token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	if !hmac.Equal([]byte(s.sign(string(raw))), []byte(parts[1])) {
		return nil, errors.New("bad signature")
	}
	bits := strings.Split(string(raw), "|")
	if len(bits) != 2 && len(bits) != 3 {
		return nil, errors.New("bad payload")
	}
	exp, _ := strconv.ParseInt(bits[1], 10, 64)
	if exp < time.Now().Unix() {
		return nil, errors.New("expired")
	}
	// Tokens issued by older builds have no version field; treat that as 0 so
	// they keep working until the user's password changes.
	var version int64
	if len(bits) == 3 {
		version, _ = strconv.ParseInt(bits[2], 10, 64)
	}
	u, err := s.Store.GetUserByID(bits[0])
	if err != nil {
		return nil, err
	}
	if u.SessionVersion != version {
		return nil, errors.New("session revoked")
	}
	return u, nil
}

func (s *Server) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := s.readSession(r)
		guestOn := s.guestEnabled()
		// Switching guest mode off must also invalidate guest sessions that were
		// already handed out, otherwise visitors keep browsing with the old cookie.
		if err == nil && !guestOn && s.isGuestID(u.ID) {
			u, err = nil, errors.New("guest session disabled")
		}
		if err != nil && guestOn {
			g, gerr := s.ensureGuest()
			if gerr == nil {
				g.Guest = true
				s.setSessionCookie(w, r, g)
				ctx := context.WithValue(r.Context(), userKey, g)
				next(w, r.WithContext(ctx))
				return
			}
			log.Printf("访客登录失败: %v", gerr)
		}
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "请先登录"})
			return
		}
		u.Guest = guestOn && s.isGuestID(u.ID)
		ctx := context.WithValue(r.Context(), userKey, u)
		next(w, r.WithContext(ctx))
	})
}

// isGuestID reports whether the id belongs to the shared guest account. It stays
// true even when guest mode is off, which is what lets stale guest sessions be
// rejected after the switch is turned off.
func (s *Server) isGuestID(id string) bool {
	if id == "" {
		return false
	}
	s.guestMu.RLock()
	cached := s.guestID
	s.guestMu.RUnlock()
	if cached != "" {
		return cached == id
	}
	return s.Store.GetSetting("guest_user_id", "") == id
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, u *store.User) {
	http.SetCookie(w, &http.Cookie{
		Name:     "mrome_session",
		Value:    s.newSession(u),
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 3600,
	})
}

func (s *Server) admin(next http.HandlerFunc) http.Handler {
	return s.auth(func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil || !u.IsAdmin {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "需要管理员权限"})
			return
		}
		next(w, r)
	})
}

func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	key := util.ClientKey(r)
	if s.loginGuard.Blocked(key) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "尝试次数过多，请 5 分钟后再试"})
		return
	}
	var req loginReq
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
	s.setSessionCookie(w, r, u)
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: "mrome_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// isHTTPS reports whether the client connection is TLS, either directly or via
// a reverse proxy that sets X-Forwarded-Proto.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	admins, _ := s.Store.CountAdmins()
	writeJSON(w, http.StatusOK, map[string]any{
		"user":      currentUser(r),
		"app":       s.appInfo(),
		"guestMode": s.guestEnabled(),
		"canLogin":  admins > 0,
	})
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	users, _ := s.Store.CountUsers()
	writeJSON(w, http.StatusOK, map[string]any{
		"app":         s.appInfo(),
		"needsSetup":  users == 0,
		"subsonicApi": "/rest",
	})
}

func (s *Server) appInfo() map[string]any {
	return map[string]any{
		"name":       "Mrome",
		"version":    "1.0.0",
		"apiVersion": "1.16.1",
		"subsonic":   true,
	}
}

// ---------------------------------------------------------------- helpers

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4<<20))
	return dec.Decode(v)
}

func qInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
