// Package audiostream implements the Synology Audio Station (DSM webapi) subset
// that music clients use: API discovery, login/logout, song/album/artist lists,
// cover art and streaming. The session id (sid) is a JWT signed with the
// instance secret, so nothing extra has to be stored server-side.
package audiostream

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

type Server struct {
	Store  *store.Store
	secret []byte
}

func New(st *store.Store) *Server {
	return &Server{Store: st, secret: []byte(st.GetSetting("secret", "mrome"))}
}

// apiInfo is what /webapi/query.cgi reports so clients can discover endpoints.
var apiInfo = map[string]any{
	"SYNO.API.Auth":              map[string]any{"path": "auth.cgi", "minVersion": 1, "maxVersion": 6},
	"SYNO.AudioStation.Song":     map[string]any{"path": "AudioStation/song.cgi", "minVersion": 1, "maxVersion": 3},
	"SYNO.AudioStation.Album":    map[string]any{"path": "AudioStation/album.cgi", "minVersion": 1, "maxVersion": 1},
	"SYNO.AudioStation.Artist":   map[string]any{"path": "AudioStation/artist.cgi", "minVersion": 1, "maxVersion": 1},
	"SYNO.AudioStation.Genre":    map[string]any{"path": "AudioStation/genre.cgi", "minVersion": 1, "maxVersion": 1},
	"SYNO.AudioStation.Playlist": map[string]any{"path": "AudioStation/playlist.cgi", "minVersion": 1, "maxVersion": 3},
	"SYNO.AudioStation.Stream":   map[string]any{"path": "AudioStation/stream.cgi", "minVersion": 1, "maxVersion": 2},
	"SYNO.AudioStation.Cover":    map[string]any{"path": "AudioStation/cover.cgi", "minVersion": 1, "maxVersion": 1},
	"SYNO.AudioStation.Search":   map[string]any{"path": "AudioStation/search.cgi", "minVersion": 1, "maxVersion": 1},
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/webapi/query.cgi":
		s.query(w, r)
	case path == "/webapi/auth.cgi":
		s.auth(w, r)
	case path == "/webapi/AudioStation/song.cgi":
		s.songs(w, r)
	case path == "/webapi/AudioStation/album.cgi":
		s.albums(w, r)
	case path == "/webapi/AudioStation/artist.cgi":
		s.artists(w, r)
	case path == "/webapi/AudioStation/genre.cgi":
		s.genres(w, r)
	case path == "/webapi/AudioStation/playlist.cgi":
		s.playlists(w, r)
	case path == "/webapi/AudioStation/search.cgi":
		s.search(w, r)
	case path == "/webapi/AudioStation/stream.cgi":
		s.stream(w, r)
	case path == "/webapi/AudioStation/cover.cgi":
		s.cover(w, r)
	default:
		fail(w, 404)
	}
}

func (s *Server) query(w http.ResponseWriter, r *http.Request) {
	ok(w, apiInfo)
}

// ------------------------------------------------------------------ auth

func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Query().Get("method")
	switch method {
	case "logout":
		ok(w, map[string]any{})
		return
	case "", "login":
	default:
		fail(w, 404)
		return
	}

	account := first(r, "account", "username")
	passwd := first(r, "passwd", "password")
	u, err := s.Store.Authenticate(strings.TrimSpace(account), passwd)
	if err != nil {
		time.Sleep(300 * time.Millisecond)
		fail(w, 401)
		return
	}
	sid := util.SignJWT(s.secret, u.ID, 30*24*time.Hour)
	ok(w, map[string]any{"sid": sid, "did": "", "is_portal_port": false})
}

func (s *Server) user(r *http.Request) (*store.User, bool) {
	sid := strings.TrimSpace(r.URL.Query().Get("_sid"))
	if sid == "" {
		sid = strings.TrimSpace(r.URL.Query().Get("sid"))
	}
	if sid == "" {
		sid = util.BearerToken(r.Header.Get)
	}
	if sid == "" {
		return nil, false
	}
	uid, valid := util.ParseJWT(sid, s.secret)
	if !valid {
		return nil, false
	}
	u, err := s.Store.GetUserByID(uid)
	if err != nil {
		return nil, false
	}
	return u, true
}

// ------------------------------------------------------------------ lists

func (s *Server) songs(w http.ResponseWriter, r *http.Request) {
	u, valid := s.user(r)
	if !valid {
		fail(w, 119)
		return
	}
	q := r.URL.Query()
	limit := atoi(q.Get("limit"), 200)
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	offset := atoi(q.Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	albumName := first(r, "album")
	artistName := first(r, "artist")

	f := store.SongFilter{Limit: limit, Offset: offset, Search: first(r, "keyword")}
	songs, _ := s.Store.ListSongs(u.ID, f)
	total, _ := s.Store.CountSongsFiltered(u.ID, f)

	out := make([]any, 0, len(songs))
	for _, song := range songs {
		if albumName != "" && !strings.EqualFold(song.Album, albumName) {
			continue
		}
		if artistName != "" && !strings.EqualFold(song.Artist, artistName) {
			continue
		}
		out = append(out, s.songDto(song))
	}
	ok(w, map[string]any{"total": total, "offset": offset, "songs": out})
}

func (s *Server) albums(w http.ResponseWriter, r *http.Request) {
	u, valid := s.user(r)
	if !valid {
		fail(w, 119)
		return
	}
	limit := atoi(r.URL.Query().Get("limit"), 200)
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	albums, _ := s.Store.ListAlbums(u.ID, store.AlbumFilter{
		Limit: limit, Offset: atoi(r.URL.Query().Get("offset"), 0), Search: first(r, "keyword"),
	})
	out := make([]any, 0, len(albums))
	for _, a := range albums {
		out = append(out, map[string]any{
			"name":         a.Name,
			"artist":       a.Artist,
			"album_artist": a.Artist,
			"song_count":   a.SongCount,
			"duration":     a.Duration,
			"year":         a.Year,
			"genre":        a.Genre,
			"additional": map[string]any{
				"avg_song_bitrate": 0,
				"tag":              map[string]any{"artist": a.Artist, "album": a.Name},
			},
		})
	}
	ok(w, map[string]any{"total": len(out), "offset": 0, "albums": out})
}

func (s *Server) artists(w http.ResponseWriter, r *http.Request) {
	u, valid := s.user(r)
	if !valid {
		fail(w, 119)
		return
	}
	artists, _ := s.Store.ListArtists(u.ID)
	out := make([]any, 0, len(artists))
	for _, a := range artists {
		out = append(out, map[string]any{
			"name":        a.Name,
			"song_count":  a.SongCount,
			"album_count": a.AlbumCount,
			"additional":  map[string]any{"tag": map[string]any{"artist": a.Name}},
		})
	}
	ok(w, map[string]any{"total": len(out), "offset": 0, "artists": out})
}

func (s *Server) genres(w http.ResponseWriter, r *http.Request) {
	if _, valid := s.user(r); !valid {
		fail(w, 119)
		return
	}
	genres, _ := s.Store.Genres()
	out := make([]any, 0, len(genres))
	for _, g := range genres {
		out = append(out, map[string]any{"name": g.Name, "song_count": g.SongCount, "album_count": g.AlbumCount})
	}
	ok(w, map[string]any{"total": len(out), "offset": 0, "genres": out})
}

func (s *Server) playlists(w http.ResponseWriter, r *http.Request) {
	u, valid := s.user(r)
	if !valid {
		fail(w, 119)
		return
	}
	lists, _ := s.Store.ListPlaylists(u.ID)
	out := make([]any, 0, len(lists))
	for _, p := range lists {
		out = append(out, map[string]any{
			"id":         "playlist_" + p.ID,
			"name":       p.Name,
			"song_count": p.SongCount,
			"duration":   p.Duration,
			"owner":      p.Owner,
		})
	}
	ok(w, map[string]any{"total": len(out), "offset": 0, "playlists": out})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	u, valid := s.user(r)
	if !valid {
		fail(w, 119)
		return
	}
	keyword := first(r, "keyword")
	songs, _ := s.Store.ListSongs(u.ID, store.SongFilter{Search: keyword, Limit: 100})
	out := make([]any, 0, len(songs))
	for _, song := range songs {
		out = append(out, s.songDto(song))
	}
	ok(w, map[string]any{"songs": out, "total": len(out)})
}

// ------------------------------------------------------------------ media

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	if _, valid := s.user(r); !valid {
		fail(w, 119)
		return
	}
	id := strings.TrimPrefix(first(r, "id"), "music_")
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

func (s *Server) cover(w http.ResponseWriter, r *http.Request) {
	if _, valid := s.user(r); !valid {
		fail(w, 119)
		return
	}
	id := strings.TrimPrefix(first(r, "id"), "music_")
	path, mime := s.resolveCover(id)
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

func (s *Server) resolveCover(id string) (string, string) {
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

// ------------------------------------------------------------------ dto

func (s *Server) songDto(song store.Song) map[string]any {
	return map[string]any{
		"id":           "music_" + song.ID,
		"path":         song.Artist + "/" + song.Album + "/" + song.Title,
		"title":        song.Title,
		"artist":       song.Artist,
		"album":        song.Album,
		"album_artist": song.AlbumArtist,
		"genre":        song.Genre,
		"duration":     int(song.Duration + 0.5),
		"track":        song.Track,
		"disc":         song.Disc,
		"year":         song.Year,
		"type":         "file",
		"additional": map[string]any{
			"song_tag": map[string]any{
				"album":        song.Album,
				"artist":       song.Artist,
				"album_artist": song.AlbumArtist,
				"genre":        song.Genre,
				"track":        song.Track,
				"disc":         song.Disc,
				"year":         song.Year,
			},
			"song_audio": map[string]any{
				"bitrate":  song.Bitrate * 1000,
				"duration": int(song.Duration + 0.5),
				"filesize": song.Size,
			},
		},
	}
}

// ------------------------------------------------------------------ helpers

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, 200, map[string]any{"success": true, "data": data})
}

func fail(w http.ResponseWriter, code int) {
	writeJSON(w, 200, map[string]any{"success": false, "error": map[string]any{"code": code}})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func first(r *http.Request, keys ...string) string {
	for _, k := range keys {
		if v := r.URL.Query().Get(k); v != "" {
			return v
		}
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		for _, k := range keys {
			if v := r.PostForm.Get(k); v != "" {
				return v
			}
		}
	}
	return ""
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
