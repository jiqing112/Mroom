// Package jellyfin speaks the Jellyfin / Emby REST API well enough for mobile
// music clients to sign in, browse the library and stream audio.
//
// Jellyfin and Emby share almost the same surface, so one implementation serves
// both: Jellyfin mounts at the site root, Emby under /emby.
package jellyfin

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mrome/internal/store"
	"mrome/internal/util"
)

const (
	serverName    = "Mrome"
	serverVersion = "10.9.11"
	productName   = "Jellyfin Server"
)

// MountPaths are the root-level prefixes Jellyfin clients hit. Registering them
// keeps the SPA fallback from swallowing the API (all are case-distinct from the
// web routes, which are lowercase).
var MountPaths = []string{
	"/System/",
	"/Users/",
	"/Users",
	"/Items/",
	"/Items",
	"/Audio/",
	"/Artists/",
	"/Artists",
	"/MusicGenres",
	"/UserViews",
	"/Sessions/",
	"/Sessions",
	"/UserFavoriteItems/",
	"/UserItems/",
	"/DisplayPreferences/",
	"/Branding/",
	"/Plugins",
	"/QuickConnect/",
}

type Server struct {
	Store    *store.Store
	secret   []byte
	serverID string
	prefix   string
}

func New(st *store.Store, prefix string) *Server {
	secret := []byte(st.GetSetting("secret", "mrome"))
	return &Server{
		Store:    st,
		secret:   secret,
		serverID: util.MD5Hex("mrome-jellyfin-server", string(secret)),
		prefix:   prefix,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if s.prefix != "" {
		path = strings.TrimPrefix(path, s.prefix)
	}
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}

	switch {
	case path == "/System/Info/Public":
		s.infoPublic(w)
	case path == "/System/Info":
		s.info(w, r)
	case path == "/System/Ping":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Jellyfin Server"))
	case path == "/Users/AuthenticateByName":
		s.authenticate(w, r)
	case path == "/Users/Me":
		s.me(w, r)
	case path == "/Users":
		s.listUsers(w, r)
	case strings.HasPrefix(path, "/Users/"):
		// /Users/{id}/FavoriteItems/{itemId} and /Users/{id}/PlayedItems/{itemId}
		if id, ok := segmentAfter(path, "FavoriteItems"); ok {
			s.favorite(w, r, id)
			return
		}
		if id, ok := segmentAfter(path, "PlayedItems"); ok {
			s.played(w, r, id)
			return
		}
		s.userByID(w, r, strings.TrimPrefix(path, "/Users/"))
	case strings.HasPrefix(path, "/UserFavoriteItems/"):
		s.favorite(w, r, normID(strings.Split(strings.TrimPrefix(path, "/UserFavoriteItems/"), "/")[0]))
	case path == "/Items":
		s.items(w, r)
	case strings.HasPrefix(path, "/Items/"):
		s.itemPath(w, r, strings.TrimPrefix(path, "/Items/"))
	case strings.HasPrefix(path, "/Audio/"):
		s.audioPath(w, r, strings.TrimPrefix(path, "/Audio/"))
	case path == "/Artists" || path == "/Artists/AlbumArtists":
		s.artists(w, r)
	case strings.HasPrefix(path, "/Artists/"):
		s.artist(w, r, strings.TrimPrefix(path, "/Artists/"))
	case path == "/MusicGenres":
		s.genres(w, r)
	case path == "/UserViews":
		s.userViews(w, r)
	case strings.HasPrefix(path, "/Sessions/Playing"):
		s.playing(w, r)
	case path == "/Sessions":
		writeJSON(w, 200, map[string]any{})
	case strings.HasPrefix(path, "/DisplayPreferences/"):
		writeJSON(w, 200, map[string]any{"CustomPrefs": map[string]any{}})
	case path == "/Branding/Configuration":
		writeJSON(w, 200, map[string]any{"SplashscreenEnabled": false, "LoginDisclaimer": ""})
	case path == "/Plugins":
		writeJSON(w, 200, []any{})
	case strings.HasPrefix(path, "/QuickConnect/"):
		writeJSON(w, 404, map[string]any{"error": "QuickConnect 未启用"})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "接口不存在"})
	}
}

// ------------------------------------------------------------------ auth

func (s *Server) tokenUser(r *http.Request) (*store.User, bool) {
	tok := strings.TrimSpace(r.Header.Get("X-Emby-Token"))
	if tok == "" {
		tok = strings.TrimSpace(r.Header.Get("X-MediaBrowser-Token"))
	}
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("api_key"))
	}
	if tok == "" {
		if a := r.Header.Get("Authorization"); strings.Contains(a, `Token="`) {
			if i := strings.Index(a, `Token="`); i >= 0 {
				rest := a[i+7:]
				if j := strings.Index(rest, `"`); j > 0 {
					tok = rest[:j]
				}
			}
		}
	}
	if tok == "" {
		return nil, false
	}
	uid, ok := util.ParseJWT(tok, s.secret)
	if !ok {
		return nil, false
	}
	u, err := s.Store.GetUserByID(uid)
	if err != nil {
		return nil, false
	}
	return u, true
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"Username"`
		Pw       string `json:"Pw"`
		Password string `json:"Password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]any{"error": "请求格式错误"})
		return
	}
	if req.Pw == "" {
		req.Pw = req.Password
	}
	u, err := s.Store.Authenticate(strings.TrimSpace(req.Username), req.Pw)
	if err != nil {
		time.Sleep(300 * time.Millisecond)
		writeJSON(w, 401, map[string]any{"error": "用户名或密码错误"})
		return
	}
	token := util.SignJWT(s.secret, u.ID, 30*24*time.Hour)
	writeJSON(w, 200, map[string]any{
		"User":        s.userDto(u),
		"SessionInfo": map[string]any{"Id": util.RandomHex(16), "UserId": u.ID, "UserName": u.Username},
		"AccessToken": token,
		"ServerId":    s.serverID,
	})
}

func (s *Server) userDto(u *store.User) map[string]any {
	return map[string]any{
		"Id":                    u.ID,
		"Name":                  u.Username,
		"ServerId":              s.serverID,
		"HasPassword":           true,
		"HasConfiguredPassword": true,
		"PrimaryImageTag":       "",
		"LastLoginDate":         time.Now().UTC().Format("2006-01-02T15:04:05.0000000Z"),
		"Policy": map[string]any{
			"IsAdministrator":                u.IsAdmin,
			"IsHidden":                       false,
			"IsDisabled":                     false,
			"EnableAllFolders":               true,
			"EnableMediaPlayback":            true,
			"EnableAudioPlaybackTranscoding": false,
			"EnableContentDownloading":       true,
		},
		"Configuration": map[string]any{"PlayDefaultAudioTrack": true},
	}
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	writeJSON(w, 200, s.userDto(u))
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, _ := s.Store.ListUsers()
	out := make([]any, 0, len(users))
	for i := range users {
		out = append(out, s.userDto(&users[i]))
	}
	writeJSON(w, 200, out)
}

// ------------------------------------------------------------------ items

func (s *Server) items(w http.ResponseWriter, r *http.Request) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	q := r.URL.Query()
	userID := u.ID
	if v := q.Get("userId"); v != "" {
		userID = v
	}
	limit := atoi(q.Get("Limit"), 200)
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	start := atoi(q.Get("StartIndex"), 0)
	if start < 0 {
		start = 0
	}
	types := strings.ToLower(q.Get("IncludeItemTypes"))
	search := strings.TrimSpace(q.Get("SearchTerm"))
	parent := normID(q.Get("ParentId"))
	favorites := strings.Contains(strings.ToLower(q.Get("Filters")), "isfavorite")

	var items []any
	total := 0

	wantSongs := types == "" || strings.Contains(types, "audio") || strings.Contains(types, "musicvideo")
	wantAlbums := strings.Contains(types, "musicalbum")
	wantArtists := strings.Contains(types, "musicartist")

	// A ParentId pointing at an album means "songs of that album".
	if parent != "" && wantSongs {
		if a, err := s.Store.GetAlbum(userID, parent, true); err == nil {
			for _, song := range a.Songs {
				items = append(items, s.songItem(song))
			}
			writeJSON(w, 200, map[string]any{"Items": items, "TotalRecordCount": len(items), "StartIndex": 0})
			return
		}
	}

	if wantArtists && types != "" && !wantSongs && !wantAlbums {
		artists, _ := s.Store.ListArtists(userID)
		total = len(artists)
		items = s.sliceArtists(artists, start, limit)
	} else if wantAlbums && !wantSongs {
		albums, _ := s.Store.ListAlbums(userID, store.AlbumFilter{Search: search, Limit: limit, Offset: start})
		if favorites {
			albums, _ = s.Store.StarredAlbums(userID)
			total = len(albums)
		} else {
			all, _ := s.Store.ListAlbums(userID, store.AlbumFilter{Search: search, Limit: 0})
			total = len(all)
		}
		for _, a := range albums {
			items = append(items, s.albumItem(a))
		}
	} else {
		f := store.SongFilter{Search: search, Limit: limit, Offset: start}
		if favorites {
			f.StarredOnly = true
		}
		songs, _ := s.Store.ListSongs(userID, f)
		total, _ = s.Store.CountSongsFiltered(userID, f)
		for _, song := range songs {
			items = append(items, s.songItem(song))
		}
	}
	if items == nil {
		items = []any{}
	}
	writeJSON(w, 200, map[string]any{"Items": items, "TotalRecordCount": total, "StartIndex": start})
}

func (s *Server) sliceArtists(list []store.Artist, start, limit int) []any {
	out := []any{}
	for i := start; i < len(list) && len(out) < limit; i++ {
		out = append(out, s.artistItem(list[i]))
	}
	return out
}

func (s *Server) itemPath(w http.ResponseWriter, r *http.Request, rest string) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	parts := strings.Split(rest, "/")
	id := normID(parts[0])
	if len(parts) >= 3 && strings.EqualFold(parts[1], "Images") {
		s.image(w, r, id)
		return
	}
	if len(parts) >= 2 && strings.EqualFold(parts[1], "Similar") {
		writeJSON(w, 200, map[string]any{"Items": []any{}, "TotalRecordCount": 0})
		return
	}
	// Resolve the id across the three item kinds; song ids win.
	if song, err := s.Store.GetSong(u.ID, id); err == nil {
		writeJSON(w, 200, s.songItem(*song))
		return
	}
	if album, err := s.Store.GetAlbum(u.ID, id, false); err == nil {
		writeJSON(w, 200, s.albumItem(*album))
		return
	}
	if artist, err := s.Store.GetArtist(u.ID, id); err == nil {
		writeJSON(w, 200, s.artistItem(*artist))
		return
	}
	writeJSON(w, 404, map[string]any{"error": "条目不存在"})
}

func (s *Server) artists(w http.ResponseWriter, r *http.Request) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	artists, _ := s.Store.ListArtists(u.ID)
	items := make([]any, 0, len(artists))
	for _, a := range artists {
		items = append(items, s.artistItem(a))
	}
	writeJSON(w, 200, map[string]any{"Items": items, "TotalRecordCount": len(items)})
}

func (s *Server) artist(w http.ResponseWriter, r *http.Request, rest string) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	id := normID(strings.Split(rest, "/")[0])
	artist, err := s.Store.GetArtist(u.ID, id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "艺术家不存在"})
		return
	}
	writeJSON(w, 200, s.artistItem(*artist))
}

func (s *Server) genres(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.tokenUser(r); !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	genres, _ := s.Store.Genres()
	items := make([]any, 0, len(genres))
	for _, g := range genres {
		items = append(items, map[string]any{
			"Id":   util.MD5Hex("genre", g.Name),
			"Name": g.Name,
			"Type": "MusicGenre",
		})
	}
	writeJSON(w, 200, map[string]any{"Items": items, "TotalRecordCount": len(items)})
}

func (s *Server) userViews(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.tokenUser(r); !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	items := []any{
		map[string]any{
			"Id": "mrome-music", "Name": "音乐库", "Type": "CollectionFolder",
			"CollectionType": "music", "IsFolder": true,
			"ImageTags": map[string]any{}, "ServerId": s.serverID,
		},
	}
	writeJSON(w, 200, map[string]any{"Items": items, "TotalRecordCount": 1})
}

func (s *Server) playing(w http.ResponseWriter, r *http.Request) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			ItemId        string `json:"ItemId"`
			PositionTicks int64  `json:"PositionTicks"`
			IsPaused      bool   `json:"IsPaused"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		// Only count a play when the client reports stopping past the intro, the
		// same threshold the web player and Subsonic endpoint use.
		if strings.HasSuffix(r.URL.Path, "/Stopped") && req.ItemId != "" && req.PositionTicks > 30*10000000 {
			_ = s.Store.Scrobble(u.ID, normID(req.ItemId), time.Now().Unix())
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// favorite maps Jellyfin's favourite endpoints onto the shared annotations table.
func (s *Server) favorite(w http.ResponseWriter, r *http.Request, id string) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	star := r.Method == http.MethodPost || r.Method == http.MethodPut
	kind := "song"
	if _, err := s.Store.GetSong(u.ID, id); err != nil {
		if _, err := s.Store.GetAlbum(u.ID, id, false); err == nil {
			kind = "album"
		} else if _, err := s.Store.GetArtist(u.ID, id); err == nil {
			kind = "artist"
		} else {
			writeJSON(w, 404, map[string]any{"error": "条目不存在"})
			return
		}
	}
	if err := s.Store.SetStar(u.ID, id, kind, star); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"IsFavorite": star})
}

// played marks an item as played (or unplayed) from the client.
func (s *Server) played(w http.ResponseWriter, r *http.Request, id string) {
	u, ok := s.tokenUser(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	if r.Method == http.MethodPost {
		_ = s.Store.Scrobble(u.ID, id, time.Now().Unix())
	}
	writeJSON(w, 200, map[string]any{})
}

func (s *Server) userByID(w http.ResponseWriter, r *http.Request, rest string) {
	if _, ok := s.tokenUser(r); !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	id := normID(strings.Split(rest, "/")[0])
	u, err := s.Store.GetUserByID(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "用户不存在"})
		return
	}
	writeJSON(w, 200, s.userDto(u))
}

// ------------------------------------------------------------------ media

func (s *Server) audioPath(w http.ResponseWriter, r *http.Request, rest string) {
	if _, ok := s.tokenUser(r); !ok {
		writeJSON(w, 401, map[string]any{"error": "未登录"})
		return
	}
	// /Audio/{id}/stream or /Audio/{id}/stream.mp3 or /Audio/{id}/universal
	parts := strings.Split(rest, "/")
	id := normID(parts[0])
	f, err := s.Store.GetSongFile(id)
	if err != nil {
		http.Error(w, "歌曲不存在", http.StatusNotFound)
		return
	}
	file, err := os.Open(f.Path)
	if err != nil {
		http.Error(w, "音频文件不可用", http.StatusNotFound)
		return
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		http.Error(w, "音频文件不可用", http.StatusNotFound)
		return
	}
	ct := f.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, filepath.Base(f.Path), st.ModTime(), file)
}

func (s *Server) image(w http.ResponseWriter, r *http.Request, id string) {
	path, mime := s.cover(id)
	if path == "" {
		http.Error(w, "无封面", http.StatusNotFound)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "无封面", http.StatusNotFound)
		return
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		http.Error(w, "无封面", http.StatusNotFound)
		return
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), file)
}

func (s *Server) cover(id string) (string, string) {
	if p, m, err := s.Store.AlbumCoverPath(id); err == nil && p != "" {
		if _, e := os.Stat(p); e == nil {
			return p, m
		}
	}
	if song, err := s.Store.GetSong("", id); err == nil && song.AlbumID != "" {
		if p, m, err := s.Store.AlbumCoverPath(song.AlbumID); err == nil && p != "" {
			if _, e := os.Stat(p); e == nil {
				return p, m
			}
		}
	}
	if p, m, err := s.Store.ArtistCover(id); err == nil && p != "" {
		if _, e := os.Stat(p); e == nil {
			return p, m
		}
	}
	return "", ""
}

// ------------------------------------------------------------------ dtos

func (s *Server) songItem(song store.Song) map[string]any {
	ticks := int64(song.Duration * 10000000)
	tag := imageTag(song.AlbumID)
	albumArtist := song.AlbumArtist
	if albumArtist == "" {
		albumArtist = song.Artist
	}
	genres := []string{}
	if song.Genre != "" {
		genres = append(genres, song.Genre)
	}
	return map[string]any{
		"Id":                   song.ID,
		"Name":                 song.Title,
		"ServerId":             s.serverID,
		"Type":                 "Audio",
		"MediaType":            "Audio",
		"IsFolder":             false,
		"Album":                song.Album,
		"AlbumId":              song.AlbumID,
		"AlbumArtist":          albumArtist,
		"AlbumArtists":         []any{map[string]any{"Id": song.ArtistID, "Name": albumArtist}},
		"Artists":              []string{song.Artist},
		"ArtistItems":          []any{map[string]any{"Id": song.ArtistID, "Name": song.Artist}},
		"RunTimeTicks":         ticks,
		"IndexNumber":          song.Track,
		"ParentIndexNumber":    song.Disc,
		"ProductionYear":       song.Year,
		"Genres":               genres,
		"ParentId":             song.AlbumID,
		"Path":                 song.Artist + "/" + song.Album + "/" + song.Title,
		"Container":            song.Suffix,
		"LocationType":         "FileSystem",
		"ImageTags":            map[string]any{"Primary": tag},
		"AlbumPrimaryImageTag": tag,
		"MediaSources": []any{map[string]any{
			"Id":                   song.ID,
			"Name":                 filepath.Base(song.Path),
			"Container":            song.Suffix,
			"Size":                 song.Size,
			"RunTimeTicks":         ticks,
			"Bitrate":              song.Bitrate * 1000,
			"Type":                 "Default",
			"SupportsDirectPlay":   true,
			"SupportsDirectStream": true,
			"MediaStreams": []any{map[string]any{
				"Type": "Audio", "Codec": song.Suffix, "Index": 0, "IsDefault": true,
			}},
		}},
		"UserData": map[string]any{
			"PlayCount":             song.PlayCount,
			"IsFavorite":            song.Starred,
			"PlaybackPositionTicks": 0,
			"Played":                song.PlayCount > 0,
		},
	}
}

func (s *Server) albumItem(a store.Album) map[string]any {
	ticks := int64(a.Duration) * 10000000
	tag := imageTag(a.ID)
	genres := []string{}
	if a.Genre != "" {
		genres = append(genres, a.Genre)
	}
	return map[string]any{
		"Id":             a.ID,
		"Name":           a.Name,
		"ServerId":       s.serverID,
		"Type":           "MusicAlbum",
		"MediaType":      "Audio",
		"IsFolder":       true,
		"AlbumArtist":    a.Artist,
		"AlbumArtists":   []any{map[string]any{"Id": a.ArtistID, "Name": a.Artist}},
		"Artists":        []string{a.Artist},
		"ArtistItems":    []any{map[string]any{"Id": a.ArtistID, "Name": a.Artist}},
		"ParentId":       a.ArtistID,
		"ChildCount":     a.SongCount,
		"SongCount":      a.SongCount,
		"RunTimeTicks":   ticks,
		"ProductionYear": a.Year,
		"Genres":         genres,
		"ImageTags":      map[string]any{"Primary": tag},
		"UserData": map[string]any{
			"PlayCount": 0, "IsFavorite": a.Starred, "PlaybackPositionTicks": 0,
		},
	}
}

func (s *Server) artistItem(a store.Artist) map[string]any {
	tag := imageTag(a.ID)
	return map[string]any{
		"Id":         a.ID,
		"Name":       a.Name,
		"ServerId":   s.serverID,
		"Type":       "MusicArtist",
		"MediaType":  "Audio",
		"IsFolder":   true,
		"ChildCount": a.AlbumCount,
		"AlbumCount": a.AlbumCount,
		"SongCount":  a.SongCount,
		"ImageTags":  map[string]any{"Primary": tag},
		"UserData":   map[string]any{"IsFavorite": a.Starred, "PlayCount": 0},
	}
}

func imageTag(id string) string {
	if id == "" {
		return ""
	}
	return util.MD5Hex("img", id)[:16]
}

// ------------------------------------------------------------------ system

func (s *Server) infoPublic(w http.ResponseWriter) {
	writeJSON(w, 200, map[string]any{
		"ServerName":             serverName,
		"Version":                serverVersion,
		"ProductName":            productName,
		"OperatingSystem":        "",
		"Id":                     s.serverID,
		"StartupWizardCompleted": true,
		"LocalAddress":           "",
	})
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"ServerName":                 serverName,
		"Version":                    serverVersion,
		"ProductName":                productName,
		"Id":                         s.serverID,
		"OperatingSystem":            "",
		"SystemUpdateLevel":          "Release",
		"HasPendingRestart":          false,
		"StartupWizardCompleted":     true,
		"OperatingSystemDisplayName": "",
		"CanSelfRestart":             false,
		"CanLaunchWebBrowser":        false,
		"WebPath":                    "",
	})
}

// ------------------------------------------------------------------ helpers

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// normID accepts the dashed GUID form some clients send and returns the bare
// lowercase hex id used by the store.
func normID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ReplaceAll(id, "-", "")
	return strings.ToLower(id)
}

// segmentAfter pulls the first path segment that follows "/<marker>/".
func segmentAfter(path, marker string) (string, bool) {
	needle := "/" + marker + "/"
	i := strings.Index(path, needle)
	if i < 0 {
		return "", false
	}
	rest := path[i+len(needle):]
	if j := strings.Index(rest, "/"); j >= 0 {
		rest = rest[:j]
	}
	rest = normID(rest)
	return rest, rest != ""
}
