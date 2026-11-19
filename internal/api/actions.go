package api

import (
	"net/http"
	"strings"
	"time"

	"mrome/internal/util"
)

func (s *Server) listPlaylists(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	lists, err := s.Store.ListPlaylists(u.ID)
	if err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"playlists": lists})
}

func (s *Server) getPlaylist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	p, err := s.Store.GetPlaylist(u.ID, r.PathValue("id"), true)
	if err != nil {
		util.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"playlist": p})
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
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "新建歌单"
	}
	p, err := s.Store.CreatePlaylist(u.ID, req.Name, req.Comment, req.Public, req.SongIDs)
	if err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"playlist": p})
}

func (s *Server) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		util.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		util.WriteJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限修改该歌单"})
		return
	}
	var req struct {
		Name    string  `json:"name"`
		Comment *string `json:"comment"`
		Public  *bool   `json:"public"`
	}
	if err := readJSON(r, &req); err != nil {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := s.Store.UpdatePlaylistMeta(id, req.Name, req.Comment, req.Public); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	full, _ := s.Store.GetPlaylist(u.ID, id, true)
	util.WriteJSON(w, http.StatusOK, map[string]any{"playlist": full})
}

func (s *Server) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		util.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		util.WriteJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限删除该歌单"})
		return
	}
	if err := s.Store.DeletePlaylist(id); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) addPlaylistSongs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		util.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		util.WriteJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限修改该歌单"})
		return
	}
	var req struct {
		SongIDs []string `json:"songIds"`
	}
	if err := readJSON(r, &req); err != nil {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "请求格式错误"})
		return
	}
	if err := s.Store.AddPlaylistSongs(id, req.SongIDs); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	full, _ := s.Store.GetPlaylist(u.ID, id, true)
	util.WriteJSON(w, http.StatusOK, map[string]any{"playlist": full})
}

func (s *Server) removePlaylistSong(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := r.PathValue("id")
	p, err := s.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		util.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "歌单不存在"})
		return
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		util.WriteJSON(w, http.StatusForbidden, map[string]any{"error": "没有权限修改该歌单"})
		return
	}
	if err := s.Store.RemovePlaylistSong(id, r.PathValue("songId")); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	full, _ := s.Store.GetPlaylist(u.ID, id, true)
	util.WriteJSON(w, http.StatusOK, map[string]any{"playlist": full})
}

func (s *Server) setStar(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Starred bool   `json:"starred"`
	}
	if err := readJSON(r, &req); err != nil || req.ID == "" {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "参数错误"})
		return
	}
	if req.Type == "" {
		req.Type = "song"
	}
	if err := s.Store.SetStar(u.ID, req.ID, req.Type, req.Starred); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "starred": req.Starred})
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var req struct {
		ID     string `json:"id"`
		Rating int    `json:"rating"`
	}
	if err := readJSON(r, &req); err != nil || req.ID == "" {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "参数错误"})
		return
	}
	if err := s.Store.SetRating(u.ID, req.ID, "song", req.Rating); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "rating": req.Rating})
}

// scrobble records a finished play. The web player authenticates with its
// session cookie, Navidrome-mode clients with their JWT; both POST here.
func (s *Server) scrobble(w http.ResponseWriter, r *http.Request) {
	uid := ""
	if u, err := s.readSession(r); err == nil {
		uid = u.ID
	} else if u, ok := s.ndCurrentUser(r); ok {
		uid = u.ID
	} else {
		util.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "请先登录"})
		return
	}
	var req struct {
		ID         string `json:"id"`
		Submission *bool  `json:"submission"`
		Time       int64  `json:"time"`
	}
	_ = readJSON(r, &req)
	if req.Submission != nil && !*req.Submission {
		// Playback start notice: nothing is recorded until the client submits.
		util.WriteJSON(w, http.StatusOK, map[string]any{})
		return
	}
	if req.ID == "" {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": "参数错误"})
		return
	}
	ts := req.Time
	if ts == 0 {
		ts = time.Now().Unix()
	}
	if err := s.Store.Scrobble(uid, req.ID, ts); err != nil {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) stars(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	songs, _ := s.Store.StarredSongs(u.ID, 200)
	albums, _ := s.Store.StarredAlbums(u.ID)
	artists, _ := s.Store.StarredArtists(u.ID)
	util.WriteJSON(w, http.StatusOK, map[string]any{"songs": songs, "albums": albums, "artists": artists})
}
