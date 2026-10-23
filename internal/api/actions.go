package api

import (
	"net/http"
	"strings"

	"mrome/internal/store"
)

func (s *Server) listPlaylists(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	lists, err := s.Store.ListPlaylists(u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlists": lists})
}

func (s *Server) getPlaylist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	p, err := s.Store.GetPlaylist(u.ID, r.PathValue("id"), true)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

func (s *Server) createPlaylist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		Name    string   `json:"name"`
		Comment string   `json:"comment"`
		Public  bool     `json:"public"`
		SongIDs []string `json:"songIds"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "新建歌单"
	}
	p, err := s.Store.CreatePlaylist(u.ID, req.Name, req.Comment, req.Public, req.SongIDs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

func (s *Server) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限修改该歌单"})
		return
	}
	var req struct {
		Name    string  `json:"name"`
		Comment *string `json:"comment"`
		Public  *bool   `json:"public"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := s.Store.UpdatePlaylistMeta(id, req.Name, req.Comment, req.Public); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	full, _ := s.Store.GetPlaylist(u.ID, id, true)
	writeJSON(w, http.StatusOK, map[string]any{"playlist": full})
}

func (s *Server) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限删除该歌单"})
		return
	}
	if err := s.Store.DeletePlaylist(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) addPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限修改该歌单"})
		return
	}
	var req struct {
		SongIDs []string `json:"songIds"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := s.Store.AddPlaylistSongs(id, req.SongIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	full, _ := s.Store.GetPlaylist(u.ID, id, true)
	writeJSON(w, http.StatusOK, map[string]any{"playlist": full})
}

func (s *Server) removePlaylistSong(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限修改该歌单"})
		return
	}
	if err := s.Store.RemovePlaylistSong(id, r.PathValue("songId")); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	full, _ := s.Store.GetPlaylist(u.ID, id, true)
	writeJSON(w, http.StatusOK, map[string]any{"playlist": full})
}

func (s *Server) setStar(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Starred bool   `json:"starred"`
	}
	if err := readJSON(r, &req); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "参数错误"})
		return
	}
	if req.Type == "" {
		req.Type = "song"
	}
	if err := s.Store.SetStar(u.ID, req.ID, req.Type, req.Starred); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "starred": req.Starred})
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		ID     string `json:"id"`
		Rating int    `json:"rating"`
	}
	if err := readJSON(r, &req); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "参数错误"})
		return
	}
	if err := s.Store.SetRating(u.ID, req.ID, req.Rating); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rating": req.Rating})
}

func (s *Server) scrobble(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		ID   string `json:"id"`
		Time int64  `json:"time"`
	}
	if err := readJSON(r, &req); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "参数错误"})
		return
	}
	if err := s.Store.Scrobble(u.ID, req.ID, req.Time); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) stars(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	songs, _ := s.Store.StarredSongs(u.ID, 200)
	albums, _ := s.Store.StarredAlbums(u.ID)
	artists, _ := s.Store.StarredArtists(u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"songs": songs, "albums": albums, "artists": artists})
}

var _ = store.Song{}
