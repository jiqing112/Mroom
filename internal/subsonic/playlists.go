package subsonic

import (
	"net/http"
	"strconv"
	"strings"

	"mrome/internal/store"
)

func playlistObj(p store.Playlist) *Obj {
	o := NewObj()
	o.Set("id", p.ID)
	o.Set("name", p.Name)
	o.Set("owner", p.Owner)
	o.Set("public", p.Public)
	o.Set("songCount", p.SongCount)
	o.Set("duration", p.Duration)
	o.Set("created", isoTime(p.Created))
	o.Set("changed", isoTime(p.Changed))
	if p.CoverArt != "" {
		o.Set("coverArt", p.CoverArt)
	}
	if p.Comment != "" {
		o.Set("comment", p.Comment)
	}
	return o
}

func (h *Handler) getPlaylists(r *http.Request, u *store.User) (*Obj, error) {
	lists, err := h.Store.ListPlaylists(u.ID)
	if err != nil {
		return nil, err
	}
	arr := make([]*Obj, 0, len(lists))
	for _, p := range lists {
		arr = append(arr, playlistObj(p))
	}
	return NewObj().Set("playlists", NewObj().Set("playlist", arr)), nil
}

func (h *Handler) getPlaylist(r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	p, err := h.Store.GetPlaylist(u.ID, id, true)
	if err != nil {
		return nil, fail(errNotFound, "歌单不存在")
	}
	o := playlistObj(*p)
	o.Set("entry", h.songList(p.Entries))
	return NewObj().Set("playlist", o), nil
}

func (h *Handler) createPlaylist(r *http.Request, u *store.User) (*Obj, error) {
	q := r.URL.Query()
	name := param(r, "name")
	songIDs := append([]string{}, q["songId"]...)
	if len(songIDs) == 0 {
		songIDs = append(songIDs, q["songIdToAdd"]...)
	}

	if existing := param(r, "playlistId"); existing != "" {
		p, err := h.Store.GetPlaylist(u.ID, existing, false)
		if err != nil {
			return nil, fail(errNotFound, "歌单不存在")
		}
		if p.OwnerID != u.ID && !u.IsAdmin {
			return nil, fail(errNotAuthorized, "没有权限修改该歌单")
		}
		if name != "" {
			_ = h.Store.UpdatePlaylistMeta(existing, name, nil, nil)
		}
		if err := h.Store.AddPlaylistSongs(existing, songIDs); err != nil {
			return nil, err
		}
		full, err := h.Store.GetPlaylist(u.ID, existing, true)
		if err != nil {
			return nil, err
		}
		o := playlistObj(*full)
		o.Set("entry", h.songList(full.Entries))
		return NewObj().Set("playlist", o), nil
	}

	if name == "" {
		return nil, fail(errMissingParam, "缺少参数 name")
	}
	p, err := h.Store.CreatePlaylist(u.ID, name, "", true, songIDs)
	if err != nil {
		return nil, err
	}
	o := playlistObj(*p)
	o.Set("entry", h.songList(p.Entries))
	return NewObj().Set("playlist", o), nil
}

func (h *Handler) updatePlaylist(r *http.Request, u *store.User) (*Obj, error) {
	q := r.URL.Query()
	id := param(r, "playlistId")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 playlistId")
	}
	p, err := h.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		return nil, fail(errNotFound, "歌单不存在")
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		return nil, fail(errNotAuthorized, "没有权限修改该歌单")
	}

	var public *bool
	if v := param(r, "public"); v != "" {
		b := v == "true" || v == "1"
		public = &b
	}
	var comment *string
	if q.Has("comment") {
		c := q.Get("comment")
		comment = &c
	}
	if err := h.Store.UpdatePlaylistMeta(id, param(r, "name"), comment, public); err != nil {
		return nil, err
	}
	if add := q["songIdToAdd"]; len(add) > 0 {
		if err := h.Store.AddPlaylistSongs(id, add); err != nil {
			return nil, err
		}
	}
	var indexes []int
	for _, raw := range q["songIndexToRemove"] {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			indexes = append(indexes, n)
		}
	}
	if len(indexes) > 0 {
		if err := h.Store.RemovePlaylistIndexes(id, indexes); err != nil {
			return nil, err
		}
	}
	return NewObj(), nil
}

func (h *Handler) deletePlaylist(r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	p, err := h.Store.GetPlaylist(u.ID, id, false)
	if err != nil {
		return nil, fail(errNotFound, "歌单不存在")
	}
	if p.OwnerID != u.ID && !u.IsAdmin {
		return nil, fail(errNotAuthorized, "没有权限删除该歌单")
	}
	if err := h.Store.DeletePlaylist(id); err != nil {
		return nil, err
	}
	return NewObj(), nil
}
