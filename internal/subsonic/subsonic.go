package subsonic

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mrome/internal/ratelimit"
	"mrome/internal/scanner"
	"mrome/internal/store"
	"mrome/internal/util"
)

const (
	apiVersion    = "1.16.1"
	serverName    = "mrome"
	serverVersion = "1.0.0"
)

// Subsonic error codes.
const (
	errGeneric       = 0
	errMissingParam  = 10
	errAuth          = 40
	errNotAuthorized = 50
	errNotFound      = 70
)

type apiError struct {
	Code    int
	Message string
}

func (e *apiError) Error() string { return e.Message }

func fail(code int, msg string) error { return &apiError{Code: code, Message: msg} }

type Handler struct {
	Store   *store.Store
	Scanner *scanner.Scanner
	secret  []byte
	guard   *ratelimit.Guard
}

func New(st *store.Store, sc *scanner.Scanner) *Handler {
	return &Handler{
		Store:   st,
		Scanner: sc,
		secret:  []byte(st.GetSetting("secret", "mrome")),
		guard:   ratelimit.New(12, 5*time.Minute),
	}
}

func setCORS(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET, POST, HEAD, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Range, X-Requested-With")
	h.Set("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges, ETag")
	h.Set("Access-Control-Max-Age", "86400")
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	method := strings.TrimPrefix(r.URL.Path, "/rest/")
	method = strings.TrimSuffix(method, "/")
	method = strings.TrimSuffix(method, ".view")
	if method == "" || strings.Contains(method, "/") {
		h.writeError(w, r, errNotFound, "接口不存在")
		return
	}

	key := util.ClientKey(r)
	if h.guard.Blocked(key) {
		h.writeError(w, r, errAuth, "尝试次数过多，请稍后再试")
		return
	}

	u, err := h.authenticate(r)
	if err != nil {
		ae, ok := err.(*apiError)
		if !ok {
			ae = &apiError{Code: errAuth, Message: "认证失败"}
		}
		if ae.Code == errAuth {
			h.guard.Fail(key)
		}
		h.writeError(w, r, ae.Code, ae.Message)
		return
	}
	h.guard.Reset(key)

	body, herr := h.route(w, method, r, u)
	if herr != nil {
		if ae, ok := herr.(*apiError); ok {
			h.writeError(w, r, ae.Code, ae.Message)
			return
		}
		h.writeError(w, r, errGeneric, herr.Error())
		return
	}
	if body == nil {
		return // handler already wrote the response (binary payload)
	}
	h.writeOK(w, r, body)
}

func (h *Handler) route(w http.ResponseWriter, method string, r *http.Request, u *store.User) (*Obj, error) {
	switch method {
	case "ping":
		return NewObj(), nil
	case "getLicense":
		return NewObj().Set("license", NewObj().Set("valid", true)), nil
	case "getOpenSubsonicExtensions":
		return NewObj().Set("openSubsonicExtensions", namedList{
			name: "extension",
			items: []*Obj{
				NewObj().Set("name", "apiKeyAuthentication").Set("versions", []int{1}),
				NewObj().Set("name", "songLyrics").Set("versions", []int{1}),
				NewObj().Set("name", "formPost").Set("versions", []int{1}),
			},
		}), nil
	case "getMusicFolders":
		return NewObj().Set("musicFolders", NewObj().Set("musicFolder", []*Obj{
			NewObj().Set("id", 1).Set("name", "音乐库"),
		})), nil
	case "getArtists":
		return h.getArtists(r, u)
	case "getIndexes":
		return h.getIndexes(r, u)
	case "getArtist":
		return h.getArtist(r, u)
	case "getAlbum":
		return h.getAlbum(r, u)
	case "getSong":
		return h.getSong(r, u)
	case "getAlbumList", "getAlbumList2":
		return h.getAlbumList(r, u, method)
	case "getRandomSongs":
		return h.getRandomSongs(r, u)
	case "getSongsByGenre":
		return h.getSongsByGenre(r, u)
	case "getGenres":
		return h.getGenres(r, u)
	case "search2", "search3":
		return h.search(r, u, method)
	case "getStarred", "getStarred2":
		return h.getStarred(r, u, method)
	case "star":
		return h.setStar(r, u, true)
	case "unstar":
		return h.setStar(r, u, false)
	case "setRating":
		return h.setRating(r, u)
	case "scrobble":
		return h.scrobble(r, u)
	case "getPlaylists":
		return h.getPlaylists(r, u)
	case "getPlaylist":
		return h.getPlaylist(r, u)
	case "createPlaylist":
		return h.createPlaylist(r, u)
	case "updatePlaylist":
		return h.updatePlaylist(r, u)
	case "deletePlaylist":
		return h.deletePlaylist(r, u)
	case "stream":
		return h.stream(w, r, u)
	case "download":
		return h.download(w, r, u)
	case "getCoverArt":
		return h.getCoverArt(w, r, u)
	case "getLyricsBySongId":
		return h.getLyricsBySongID(r, u)
	case "getLyrics":
		return h.getLyrics(r, u)
	case "getUser":
		return h.getUser(r, u)
	case "getUsers":
		return h.getUsers(r, u)
	case "getScanStatus":
		return h.getScanStatus(r, u)
	case "startScan":
		return h.startScan(r, u)
	case "getNowPlaying":
		return NewObj().Set("nowPlaying", NewObj().Set("entry", []*Obj{})), nil
	case "getArtistInfo", "getArtistInfo2":
		return h.getArtistInfo(r, u, method)
	case "getAlbumInfo", "getAlbumInfo2":
		return h.getAlbumInfo(r, u, method)
	case "getTopSongs":
		return NewObj().Set("topSongs", NewObj().Set("song", []*Obj{})), nil
	case "getSimilarSongs", "getSimilarSongs2":
		return NewObj().Set("similarSongs"+strings.TrimPrefix(method, "getSimilarSongs"), NewObj().Set("song", []*Obj{})), nil
	case "getPlayQueue":
		return NewObj().Set("playQueue", NewObj()), nil
	case "getBookmarks":
		return NewObj().Set("bookmarks", NewObj().Set("bookmark", []*Obj{})), nil
	case "getShares":
		return NewObj().Set("shares", NewObj().Set("share", []*Obj{})), nil
	case "getChatMessages":
		return NewObj().Set("chatMessages", NewObj().Set("chatMessage", []*Obj{})), nil
	case "getVideos", "getVideoInfo", "getCaptions", "getAvatar", "jukeboxControl", "getPodcasts", "getNewestPodcasts":
		return nil, fail(errNotFound, "该接口未实现（mrome 专注音乐播放）")
	default:
		return nil, fail(errGeneric, "未知接口: "+method)
	}
}

// ---------------------------------------------------------------- auth

func (h *Handler) authenticate(r *http.Request) (*store.User, error) {
	q := r.URL.Query()
	apiKey := strings.TrimSpace(q.Get("apiKey"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if apiKey != "" {
		u, err := h.Store.UserByAPIKey(apiKey)
		if err != nil {
			return nil, fail(errAuth, "API Key 无效")
		}
		return u, nil
	}

	// Clients in "Navidrome" mode authenticate once at /auth/login and then
	// present the JWT on every request instead of u/p/t/s.
	token := util.BearerToken(r.Header.Get)
	if token == "" {
		token = strings.TrimSpace(q.Get("jwt"))
	}
	if token != "" {
		if uid, ok := util.ParseJWT(token, h.secret); ok {
			if u, err := h.Store.GetUserByID(uid); err == nil {
				return u, nil
			}
		}
		if q.Get("u") == "" {
			return nil, fail(errAuth, "登录已过期，请重新登录")
		}
	}

	username := strings.TrimSpace(q.Get("u"))
	if username == "" {
		return nil, fail(errMissingParam, "缺少参数 u")
	}

	if token := q.Get("t"); token != "" {
		salt := q.Get("s")
		if salt == "" {
			return nil, fail(errMissingParam, "使用 token 认证时必须提供参数 s")
		}
		u, err := h.Store.CheckSubsonicToken(username, token, salt)
		if err != nil {
			return nil, fail(errAuth, "用户名或密码错误")
		}
		return u, nil
	}

	if p := q.Get("p"); p != "" {
		if strings.HasPrefix(p, "enc:") {
			if b, err := hexDecode(strings.TrimPrefix(p, "enc:")); err == nil {
				p = string(b)
			}
		}
		u, err := h.Store.Authenticate(username, p)
		if err != nil {
			return nil, fail(errAuth, "用户名或密码错误")
		}
		return u, nil
	}

	return nil, fail(errMissingParam, "缺少认证参数 t/s 或 p")
}

// ---------------------------------------------------------------- envelope

func (h *Handler) writeOK(w http.ResponseWriter, r *http.Request, body *Obj) {
	h.writeEnvelope(w, r, body, 0, "")
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	h.writeEnvelope(w, r, nil, code, msg)
}

func (h *Handler) writeEnvelope(w http.ResponseWriter, r *http.Request, body *Obj, code int, msg string) {
	resp := NewObj()
	ok := code == 0
	if ok {
		resp.Set("status", "ok")
	} else {
		resp.Set("status", "failed")
	}
	resp.Set("version", apiVersion)
	resp.Set("type", serverName)
	resp.Set("serverVersion", serverVersion)
	resp.Set("openSubsonic", true)
	if body != nil {
		for _, k := range body.keys {
			resp.Set(k, body.vals[k])
		}
	}
	if !ok {
		resp.Set("error", NewObj().Set("code", code).Set("message", msg))
	}

	format := strings.ToLower(r.URL.Query().Get("f"))
	if format == "json" || format == "jsonp" {
		payload := map[string]*Obj{"subsonic-response": resp}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(payload)
		if format == "jsonp" {
			if cb := jsonpCallback(r.URL.Query().Get("callback")); cb != "" {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(cb + "("))
				_, _ = w.Write(bytes.TrimSpace(buf.Bytes()))
				_, _ = w.Write([]byte(");\n"))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
		return
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	start := xml.StartElement{
		Name: xml.Name{Local: "subsonic-response"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "xmlns"}, Value: "http://subsonic.org/restapi"}},
	}
	if err := enc.EncodeElement(resp, start); err != nil {
		log.Printf("subsonic xml encode: %v", err)
	}
	_ = enc.Flush()
}

// ---------------------------------------------------------------- builders

func isoTime(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func songObj(s store.Song) *Obj {
	o := NewObj()
	o.Set("id", s.ID)
	o.Set("parent", s.AlbumID)
	o.Set("isDir", false)
	o.Set("title", s.Title)
	o.Set("album", s.Album)
	o.Set("artist", s.Artist)
	o.Set("track", s.Track)
	o.Set("year", s.Year)
	o.Set("genre", s.Genre)
	o.Set("coverArt", s.CoverArt)
	o.Set("size", s.Size)
	o.Set("contentType", s.ContentType)
	o.Set("suffix", s.Suffix)
	o.Set("duration", int(s.Duration+0.5))
	o.Set("bitRate", s.Bitrate)
	o.Set("path", s.Artist+"/"+s.Album+"/"+s.Title)
	o.Set("albumId", s.AlbumID)
	o.Set("artistId", s.ArtistID)
	o.Set("type", "music")
	o.Set("created", isoTime(s.Created))
	if s.Starred {
		starredAt := s.StarredAt
		if starredAt == 0 {
			starredAt = s.Created
		}
		o.Set("starred", isoTime(starredAt))
	}
	if s.Rating > 0 {
		o.Set("userRating", s.Rating)
	}
	if s.PlayCount > 0 {
		o.Set("playCount", s.PlayCount)
	}
	if s.Disc > 0 {
		o.Set("discNumber", s.Disc)
	}
	return o
}

func albumObj(a store.Album) *Obj {
	o := NewObj()
	o.Set("id", a.ID)
	o.Set("name", a.Name)
	o.Set("artist", a.Artist)
	o.Set("artistId", a.ArtistID)
	o.Set("coverArt", a.CoverArt)
	o.Set("songCount", a.SongCount)
	o.Set("duration", a.Duration)
	o.Set("created", isoTime(a.Created))
	o.Set("year", a.Year)
	o.Set("genre", a.Genre)
	if a.Starred {
		starredAt := a.StarredAt
		if starredAt == 0 {
			starredAt = a.Created
		}
		o.Set("starred", isoTime(starredAt))
	}
	if a.Rating > 0 {
		o.Set("userRating", a.Rating)
	}
	return o
}

func artistObj(a store.Artist) *Obj {
	o := NewObj()
	o.Set("id", a.ID)
	o.Set("name", a.Name)
	o.Set("albumCount", a.AlbumCount)
	if a.CoverArt != "" {
		o.Set("coverArt", a.CoverArt)
	}
	if a.Starred {
		starredAt := a.StarredAt
		if starredAt == 0 {
			starredAt = time.Now().Unix()
		}
		o.Set("starred", isoTime(starredAt))
	}
	return o
}

func (h *Handler) songList(songs []store.Song) []*Obj {
	out := make([]*Obj, 0, len(songs))
	for _, s := range songs {
		out = append(out, songObj(s))
	}
	return out
}

func (h *Handler) albumList(albums []store.Album) []*Obj {
	out := make([]*Obj, 0, len(albums))
	for _, a := range albums {
		out = append(out, albumObj(a))
	}
	return out
}

func (h *Handler) artistList(artists []store.Artist) []*Obj {
	out := make([]*Obj, 0, len(artists))
	for _, a := range artists {
		out = append(out, artistObj(a))
	}
	return out
}

func param(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func intParam(r *http.Request, key string, def int) int {
	return util.QueryInt(r, key, def)
}

func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd-length hex string")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi, err := strconv.ParseUint(s[i*2:i*2+1], 16, 8)
		if err != nil {
			return nil, err
		}
		lo, err := strconv.ParseUint(s[i*2+1:i*2+2], 16, 8)
		if err != nil {
			return nil, err
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, nil
}

func baseName(p string) string { return filepath.Base(p) }

// jsonpCallback accepts simple JS identifiers/paths such as jQuery123_456 or
// window.cb; anything else is rejected and the response falls back to JSON.
func jsonpCallback(name string) string {
	if name == "" || len(name) > 128 {
		return ""
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r == '$', r == '.':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return ""
		}
	}
	return name
}

func timeNowUnix() int64 { return time.Now().Unix() }

var _ = fmt.Sprintf
