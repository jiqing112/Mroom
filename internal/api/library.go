package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mrome/internal/media"
	"mrome/internal/store"
	"mrome/internal/util"
)

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	stats, _ := s.Store.Stats()
	newest, _ := s.Store.ListAlbums(u.ID, store.AlbumFilter{Sort: "newest", Limit: 14})
	recent, _ := s.Store.ListAlbums(u.ID, store.AlbumFilter{Sort: "recent", Limit: 8})
	recentSongs, _ := s.Store.ListSongs(u.ID, store.SongFilter{Sort: "created", Limit: 8})
	genres, _ := s.Store.Genres()
	if len(genres) > 8 {
		genres = genres[:8]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"stats":       stats,
		"newest":      newest,
		"recent":      recent,
		"recentSongs": recentSongs,
		"genres":      genres,
	})
}

func (s *Server) listSongs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	f := store.SongFilter{
		Search:      strings.TrimSpace(r.URL.Query().Get("q")),
		AlbumID:     r.URL.Query().Get("albumId"),
		ArtistID:    r.URL.Query().Get("artistId"),
		Genre:       r.URL.Query().Get("genre"),
		StarredOnly: r.URL.Query().Get("starred") == "true",
		Sort:        r.URL.Query().Get("sort"),
		Limit:       util.Clamp(qInt(r, "limit", 200), 1, 1000),
		Offset:      qInt(r, "offset", 0),
	}
	songs, err := s.Store.ListSongs(u.ID, f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	total, _ := s.Store.CountSongsFiltered(u.ID, f)
	writeJSON(w, http.StatusOK, map[string]any{"songs": songs, "total": total})
}

func (s *Server) recentSongs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	songs, err := s.Store.RecentlyPlayed(u.ID, util.Clamp(qInt(r, "limit", 100), 1, 500))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"songs": songs, "total": len(songs)})
}

func (s *Server) getSong(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	song, err := s.Store.GetSong(u.ID, r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌曲不存在"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"song": song})
}

func (s *Server) lyrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	raw, err := s.Store.SongLyrics(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌曲不存在"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lyrics": raw, "hasLyrics": strings.TrimSpace(raw) != ""})
}

func (s *Server) updateSong(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Artist      string `json:"artist"`
		AlbumArtist string `json:"albumArtist"`
		Album       string `json:"album"`
		Genre       string `json:"genre"`
		Year        int    `json:"year"`
		Track       int    `json:"track"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	id := r.PathValue("id")
	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "标题不能为空"})
		return
	}
	if err := s.Store.UpdateSongMeta(id, req.Title, req.Artist, req.AlbumArtist, req.Album, req.Genre, req.Year, req.Track); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	u := currentUser(r)
	song, err := s.Store.GetSong(u.ID, id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"song": song})
}

func (s *Server) deleteSong(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	file, err := s.Store.GetSongFile(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌曲不存在"})
		return
	}
	// Move the audio file into a trash folder instead of destroying it. Keep
	// the DB row until the move has actually succeeded so a failed rename
	// cannot leave an orphaned song that the next scan re-imports.
	trash := filepath.Join(s.Cfg.DataDir, "trash")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	target := uniquePath(filepath.Join(trash, filepath.Base(file.Path)))
	if err := os.Rename(file.Path, target); err != nil {
		if !os.IsNotExist(err) {
			// Cross-device moves cannot be renamed; fall back to a copy.
			if cerr := copyFile(file.Path, target); cerr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "无法移动文件到回收站: " + cerr.Error()})
				return
			}
			if rerr := os.Remove(file.Path); rerr != nil && !os.IsNotExist(rerr) {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "无法移除原文件: " + rerr.Error()})
				return
			}
		}
	}
	// The audio file has moved to the trash; its sidecar lyrics are no longer
	// reachable, so remove them too. Folder artwork is album-level and stays.
	base := strings.TrimSuffix(file.Path, filepath.Ext(file.Path))
	for _, ext := range []string{".lrc", ".LRC"} {
		_ = os.Remove(base + ext)
	}
	if err := s.Store.DeleteSong(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	_ = s.Store.RefreshAlbumStats()
	cleanupAlbumFolder(file.Path, trash)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "trashed": target})
}

var albumCoverFiles = []string{
	"cover.jpg", "cover.jpeg", "cover.png", "folder.jpg", "folder.png",
	"front.jpg", "album.jpg", "AlbumArt.jpg",
}

// cleanupAlbumFolder moves folder artwork to the trash and removes the folder
// once its last audio file (and sidecar lyrics) has been deleted.
func cleanupAlbumFolder(audioPath, trash string) {
	dir := filepath.Dir(audioPath)
	if dir == "" || dir == "." {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && media.IsAudio(filepath.Join(dir, e.Name())) {
			return
		}
	}
	for _, name := range albumCoverFiles {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			continue
		}
		target := uniquePath(filepath.Join(trash, name))
		if err := os.Rename(p, target); err != nil {
			if err := copyFile(p, target); err == nil {
				_ = os.Remove(p)
			}
		}
	}
	_ = os.Remove(dir)
}

func (s *Server) listArtists(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	artists, err := s.Store.ListArtists(u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"artists": artists})
}

func (s *Server) getArtist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	artist, err := s.Store.GetArtist(u.ID, r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "艺术家不存在"})
		return
	}
	limit := util.Clamp(qInt(r, "limit", 500), 1, 1000)
	offset := qInt(r, "offset", 0)
	filter := store.SongFilter{ArtistID: artist.ID, Sort: "album", Limit: limit, Offset: offset}
	songs, _ := s.Store.ListSongs(u.ID, filter)
	total, _ := s.Store.CountSongsFiltered(u.ID, filter)
	writeJSON(w, http.StatusOK, map[string]any{"artist": artist, "songs": songs, "total": total})
}

func (s *Server) listAlbums(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	f := store.AlbumFilter{
		Sort:        r.URL.Query().Get("sort"),
		Genre:       r.URL.Query().Get("genre"),
		ArtistID:    r.URL.Query().Get("artistId"),
		StarredOnly: r.URL.Query().Get("starred") == "true",
		Limit:       util.Clamp(qInt(r, "limit", 200), 1, 1000),
		Offset:      qInt(r, "offset", 0),
	}
	albums, err := s.Store.ListAlbums(u.ID, f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"albums": albums})
}

func (s *Server) getAlbum(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	album, err := s.Store.GetAlbum(u.ID, r.PathValue("id"), true)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "专辑不存在"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"album": album})
}

func (s *Server) listGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := s.Store.Genres()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"genres": genres})
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"artists": []any{}, "albums": []any{}, "songs": []any{}})
		return
	}
	artists, _ := s.Store.SearchArtists(u.ID, q, 6)
	albums, _ := s.Store.SearchAlbums(u.ID, q, 8)
	songs, _ := s.Store.ListSongs(u.ID, store.SongFilter{Search: q, Sort: "title", Limit: 20})
	writeJSON(w, http.StatusOK, map[string]any{"artists": artists, "albums": albums, "songs": songs})
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	f, err := s.Store.GetSongFile(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌曲不存在"})
		return
	}
	file, err := os.Open(f.Path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "音频文件不可用"})
		return
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "音频文件不可用"})
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
	id := r.PathValue("id")
	size := 0
	if raw := r.PathValue("size"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			size = util.Clamp(n, 48, 1024)
		}
	}
	path, mime := s.coverPath(id)
	if path == "" {
		// Transparent 1x1 fallback so the UI can render a placeholder gracefully.
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"))
		return
	}
	if size > 0 {
		if thumb, thumbMime, err := s.coverThumbnail(path, size); err == nil {
			path, mime = thumb, thumbMime
		}
	}
	file, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Header().Set("ETag", `"`+id+`-`+strconv.Itoa(size)+`-`+strconv.FormatInt(st.ModTime().Unix(), 10)+`"`)
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), file)
}

func (s *Server) coverPath(id string) (string, string) {
	if p, m, err := s.Store.AlbumCoverPath(id); err == nil && p != "" {
		return p, m
	}
	if song, err := s.Store.GetSong("", id); err == nil && song.AlbumID != "" {
		if p, m, err := s.Store.AlbumCoverPath(song.AlbumID); err == nil && p != "" {
			return p, m
		}
	}
	if p, m, err := s.Store.ArtistCover(id); err == nil && p != "" {
		return p, m
	}
	return "", ""
}
