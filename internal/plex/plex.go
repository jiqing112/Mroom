// Package plex implements a small, read-only slice of the Plex Media Server API
// (XML over HTTP) so a client that can be pointed at a server manually — and can
// supply a token — is able to browse and stream.
//
// Plex clients normally discover servers through plex.tv and authenticate with a
// Plex account, which a self-hosted server cannot impersonate. This package is
// therefore aimed at the "manual address + token" path: the token is a stable
// value derived from the instance secret and shown in the settings panel.
package plex

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mrome/internal/store"
	"mrome/internal/util"
)

const (
	serverName    = "Mrome"
	serverVersion = "1.40.0"
)

// MountPaths are the prefixes Plex clients request.
var MountPaths = []string{
	"/identity",
	"/library/",
	"/photo/:/transcode",
	"/:/prefs",
}

type Server struct {
	Store     *store.Store
	secret    []byte
	token     string
	machineID string
}

func New(st *store.Store) *Server {
	secret := []byte(st.GetSetting("secret", "mrome"))
	return &Server{
		Store:     st,
		secret:    secret,
		token:     util.MD5Hex("mrome-plex-token", string(secret)),
		machineID: util.MD5Hex("mrome-plex-machine", string(secret))[:32],
	}
}

// Token exposes the value clients must send as X-Plex-Token.
func (s *Server) Token() string { return s.token }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		w.Header().Set("X-Plex-Content-Protocol", "media")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><MediaContainer size="0"></MediaContainer>`))
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/identity" || path == "":
		s.identity(w)
	case path == "/library/sections":
		s.sections(w)
	case strings.HasPrefix(path, "/library/sections/"):
		s.section(w, r, strings.TrimPrefix(path, "/library/sections/"))
	case strings.HasPrefix(path, "/library/metadata/"):
		s.metadata(w, r, strings.TrimPrefix(path, "/library/metadata/"))
	case strings.HasPrefix(path, "/library/parts/"):
		s.part(w, r, strings.TrimPrefix(path, "/library/parts/"))
	case path == "/library/recentlyAdded":
		s.recentlyAdded(w, r)
	case path == "/library/onDeck" || path == "/library/hubs":
		s.empty(w)
	case path == "/photo/:/transcode":
		s.transcode(w, r)
	case path == "/:/prefs":
		s.prefs(w)
	case path == "/library":
		s.empty(w)
	default:
		s.empty(w)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	tok := strings.TrimSpace(r.Header.Get("X-Plex-Token"))
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("X-Plex-Token"))
	}
	return tok == s.token
}

// ------------------------------------------------------------------ containers

func (s *Server) identity(w http.ResponseWriter) {
	body := fmt.Sprintf(
		`<MediaContainer size="0" apiVersion="1.1" claimed="0" machineIdentifier="%s" version="%s"></MediaContainer>`,
		s.machineID, serverVersion)
	s.writeXML(w, body)
}

func (s *Server) prefs(w http.ResponseWriter) {
	s.writeXML(w, `<MediaContainer size="0"></MediaContainer>`)
}

func (s *Server) empty(w http.ResponseWriter) {
	s.writeXML(w, `<MediaContainer size="0" totalSize="0"></MediaContainer>`)
}

func (s *Server) sections(w http.ResponseWriter) {
	body := fmt.Sprintf(
		`<MediaContainer size="1" allowSync="0" identifier="com.plexapp.plugins.library">`+
			`<Directory key="1" type="artist" title="音乐库" librarySectionTitle="音乐库" `+
			`librarySectionID="1" librarySectionKey="/library/sections/1" `+
			`agent="com.plexapp.agents.none" scanner="Plex Music" language="zh" uuid="%s" updatedAt="0" createdAt="0"/>`+
			`</MediaContainer>`, s.machineID)
	s.writeXML(w, body)
}

func (s *Server) section(w http.ResponseWriter, r *http.Request, rest string) {
	// /library/sections/1/all?type=8|9|10
	parts := strings.Split(rest, "/")
	key := parts[0]
	if key != "1" {
		s.empty(w)
		return
	}
	if len(parts) < 2 || parts[1] != "all" {
		s.empty(w)
		return
	}
	kind := r.URL.Query().Get("type")
	u := s.user()
	var b strings.Builder
	switch kind {
	case "8": // tracks
		songs, _ := s.Store.ListSongs(u, store.SongFilter{Limit: 2000, Sort: "title"})
		fmt.Fprintf(&b, `<MediaContainer size="%d" totalSize="%d" offset="0"`, len(songs), len(songs))
		b.WriteString(` allowSync="0" art="/library/sections/1/art" identifier="com.plexapp.plugins.library" librarySectionID="1" librarySectionTitle="音乐库" mediaTagPrefix="/system/bundle/media/flags/" mediaTagVersion="1" title1="音乐库">`)
		for _, song := range songs {
			b.WriteString(s.trackXML(song))
		}
		b.WriteString(`</MediaContainer>`)
	case "9": // albums
		albums, _ := s.Store.ListAlbums(u, store.AlbumFilter{Limit: 2000})
		fmt.Fprintf(&b, `<MediaContainer size="%d" totalSize="%d" offset="0"`, len(albums), len(albums))
		b.WriteString(` allowSync="0" art="/library/sections/1/art" identifier="com.plexapp.plugins.library" librarySectionID="1" librarySectionTitle="音乐库" mediaTagPrefix="/system/bundle/media/flags/" mediaTagVersion="1" title1="音乐库">`)
		for _, a := range albums {
			b.WriteString(s.albumXML(a))
		}
		b.WriteString(`</MediaContainer>`)
	default: // 10 or unspecified -> artists
		artists, _ := s.Store.ListArtists(u)
		fmt.Fprintf(&b, `<MediaContainer size="%d" totalSize="%d" offset="0"`, len(artists), len(artists))
		b.WriteString(` allowSync="0" art="/library/sections/1/art" identifier="com.plexapp.plugins.library" librarySectionID="1" librarySectionTitle="音乐库" mediaTagPrefix="/system/bundle/media/flags/" mediaTagVersion="1" title1="音乐库">`)
		for _, a := range artists {
			fmt.Fprintf(&b, `<Directory ratingKey="%s" key="/library/metadata/%s/children" guid="mrome://artist/%s" type="artist" title="%s" librarySectionTitle="音乐库" librarySectionID="1" librarySectionKey="/library/sections/1" thumb="/library/metadata/%s/thumb/1" addedAt="%d" updatedAt="%d"/>`,
				a.ID, a.ID, a.ID, esc(a.Name), a.ID, 0, 0)
		}
		b.WriteString(`</MediaContainer>`)
	}
	s.writeXML(w, b.String())
}

func (s *Server) recentlyAdded(w http.ResponseWriter, r *http.Request) {
	s.section(w, r, "1/all?type=8")
}

func (s *Server) metadata(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	id := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	u := s.user()

	switch sub {
	case "thumb", "art":
		s.thumb(w, r, id)
		return
	case "children":
		// Album -> its tracks; artist -> its albums.
		if album, err := s.Store.GetAlbum(u, id, true); err == nil {
			var b strings.Builder
			fmt.Fprintf(&b, `<MediaContainer size="%d" totalSize="%d" offset="0"`, len(album.Songs), len(album.Songs))
			b.WriteString(` parentIndex="1" parentRatingKey="` + album.ID + `" parentTitle="` + esc(album.Name) + `" parentYear="` + strconv.Itoa(album.Year) + `" librarySectionID="1" title1="音乐库">`)
			for _, song := range album.Songs {
				b.WriteString(s.trackXML(song))
			}
			b.WriteString(`</MediaContainer>`)
			s.writeXML(w, b.String())
			return
		}
		if artist, err := s.Store.GetArtist(u, id); err == nil {
			var b strings.Builder
			fmt.Fprintf(&b, `<MediaContainer size="%d" totalSize="%d" offset="0"`, len(artist.Albums), len(artist.Albums))
			b.WriteString(` librarySectionID="1" title1="音乐库">`)
			for _, a := range artist.Albums {
				b.WriteString(s.albumXML(a))
			}
			b.WriteString(`</MediaContainer>`)
			s.writeXML(w, b.String())
			return
		}
		s.empty(w)
		return
	}

	if song, err := s.Store.GetSong(u, id); err == nil {
		var b strings.Builder
		b.WriteString(`<MediaContainer size="1" totalSize="1" offset="0" librarySectionID="1" title1="音乐库">`)
		b.WriteString(s.trackXML(*song))
		b.WriteString(`</MediaContainer>`)
		s.writeXML(w, b.String())
		return
	}
	if album, err := s.Store.GetAlbum(u, id, false); err == nil {
		var b strings.Builder
		b.WriteString(`<MediaContainer size="1" totalSize="1" offset="0" librarySectionID="1" title1="音乐库">`)
		b.WriteString(s.albumXML(*album))
		b.WriteString(`</MediaContainer>`)
		s.writeXML(w, b.String())
		return
	}
	if artist, err := s.Store.GetArtist(u, id); err == nil {
		var b strings.Builder
		b.WriteString(`<MediaContainer size="1" totalSize="1" offset="0" librarySectionID="1" title1="音乐库">`)
		fmt.Fprintf(&b, `<Directory ratingKey="%s" key="/library/metadata/%s/children" guid="mrome://artist/%s" type="artist" title="%s" librarySectionTitle="音乐库" librarySectionID="1" librarySectionKey="/library/sections/1" thumb="/library/metadata/%s/thumb/1" addedAt="%d" updatedAt="%d"/>`,
			artist.ID, artist.ID, artist.ID, esc(artist.Name), artist.ID, 0, 0)
		b.WriteString(`</MediaContainer>`)
		s.writeXML(w, b.String())
		return
	}
	s.empty(w)
}

// ------------------------------------------------------------------ media

func (s *Server) part(w http.ResponseWriter, r *http.Request, rest string) {
	// /library/parts/{songId}/file.mp3
	id := strings.Split(rest, "/")[0]
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

func (s *Server) thumb(w http.ResponseWriter, r *http.Request, id string) {
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

// transcode serves the cover referenced by ?url=/library/metadata/<id>/thumb
func (s *Server) transcode(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("url")
	if u == "" {
		http.Error(w, "缺少参数", http.StatusBadRequest)
		return
	}
	if i := strings.Index(u, "/library/metadata/"); i >= 0 {
		rest := u[i+len("/library/metadata/"):]
		id := strings.Split(rest, "/")[0]
		s.thumb(w, r, id)
		return
	}
	http.Error(w, "不支持", http.StatusNotFound)
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

// ------------------------------------------------------------------ xml

func (s *Server) trackXML(song store.Song) string {
	ms := int(song.Duration * 1000)
	albumArtist := song.AlbumArtist
	if albumArtist == "" {
		albumArtist = song.Artist
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<Track ratingKey="%s" key="/library/metadata/%s" guid="mrome://track/%s" type="track" title="%s" `,
		song.ID, song.ID, song.ID, esc(song.Title))
	fmt.Fprintf(&b, `summary="" index="%d" parentIndex="%d" year="%d" duration="%d" `,
		song.Track, song.Disc, song.Year, ms)
	fmt.Fprintf(&b, `parentRatingKey="%s" parentKey="/library/metadata/%s" parentTitle="%s" `,
		song.AlbumID, song.AlbumID, esc(song.Album))
	fmt.Fprintf(&b, `grandparentRatingKey="%s" grandparentKey="/library/metadata/%s" grandparentTitle="%s" `,
		song.ArtistID, song.ArtistID, esc(song.Artist))
	fmt.Fprintf(&b, `thumb="/library/metadata/%s/thumb/1" parentThumb="/library/metadata/%s/thumb/1" grandparentThumb="/library/metadata/%s/thumb/1" `,
		song.AlbumID, song.AlbumID, song.ArtistID)
	fmt.Fprintf(&b, `addedAt="%d" updatedAt="%d" viewCount="%d" `, song.Created, song.Created, song.PlayCount)
	if song.Starred {
		fmt.Fprintf(&b, `userRating="10" `)
	}
	b.WriteString(`>`)
	fmt.Fprintf(&b, `<Media id="%s" duration="%d" bitrate="%d" audioCodec="%s" container="%s" audioChannels="2" `,
		song.ID, ms, song.Bitrate, esc(song.Suffix), esc(song.Suffix))
	fmt.Fprintf(&b, `>`)
	fmt.Fprintf(&b, `<Part id="%s" key="/library/parts/%s/file.%s" duration="%d" file="%s" size="%d" container="%s"/>`,
		song.ID, song.ID, esc(song.Suffix), ms, esc(song.Artist+"/"+song.Album+"/"+song.Title), song.Size, esc(song.Suffix))
	b.WriteString(`</Media></Track>`)
	return b.String()
}

func (s *Server) albumXML(a store.Album) string {
	ms := int64(a.Duration) * 1000
	var b strings.Builder
	fmt.Fprintf(&b, `<Directory ratingKey="%s" key="/library/metadata/%s/children" guid="mrome://album/%s" type="album" title="%s" `,
		a.ID, a.ID, a.ID, esc(a.Name))
	fmt.Fprintf(&b, `parentRatingKey="%s" parentTitle="%s" year="%d" leafCount="%d" duration="%d" `,
		a.ArtistID, esc(a.Artist), a.Year, a.SongCount, ms)
	fmt.Fprintf(&b, `thumb="/library/metadata/%s/thumb/1" parentThumb="/library/metadata/%s/thumb/1" `, a.ID, a.ArtistID)
	fmt.Fprintf(&b, `addedAt="%d" updatedAt="%d"/>`, a.Created, a.Created)
	return b.String()
}

func (s *Server) writeXML(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("X-Plex-Content-Protocol", "media")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	_, _ = w.Write([]byte(body))
}

func esc(s string) string {
	return html.EscapeString(s)
}

// user returns the sole library owner; Plex has no multi-user concept here.
func (s *Server) user() string {
	users, err := s.Store.ListUsers()
	if err != nil || len(users) == 0 {
		return ""
	}
	for i := range users {
		if users[i].IsAdmin {
			return users[i].ID
		}
	}
	return users[0].ID
}
