package subsonic

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"mrome/internal/store"
	"mrome/internal/util"
)

func indexLetter(name string) string {
	for _, r := range name {
		u := unicode.ToUpper(r)
		if u >= 'A' && u <= 'Z' {
			return string(u)
		}
		return "#"
	}
	return "#"
}

func (h *Handler) getArtists(r *http.Request, u *store.User) (*Obj, error) {
	artists, err := h.Store.ListArtists(u.ID)
	if err != nil {
		return nil, err
	}
	indexes := map[string]*Obj{}
	order := []string{}
	for _, a := range artists {
		letter := indexLetter(a.Name)
		idx, ok := indexes[letter]
		if !ok {
			idx = NewObj().Set("name", letter)
			indexes[letter] = idx
			order = append(order, letter)
		}
		idx.Append("artist", artistObj(a))
	}
	return NewObj().Set("artists", NewObj().Set("ignoredArticles", "The El La Los Las Le Les").
		Set("index", sortedIndexes(indexes))), nil
}

func (h *Handler) getIndexes(r *http.Request, u *store.User) (*Obj, error) {
	artists, err := h.Store.ListArtists(u.ID)
	if err != nil {
		return nil, err
	}
	indexes := map[string]*Obj{}
	for _, a := range artists {
		letter := indexLetter(a.Name)
		idx, ok := indexes[letter]
		if !ok {
			idx = NewObj().Set("name", letter)
			indexes[letter] = idx
		}
		idx.Append("artist", NewObj().Set("id", a.ID).Set("name", a.Name))
	}
	return NewObj().Set("indexes", NewObj().
		Set("lastModified", isoTime(nowUnix())).
		Set("ignoredArticles", "The El La Los Las Le Les").
		Set("index", sortedIndexes(indexes))), nil
}

func (h *Handler) getArtist(r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	artist, err := h.Store.GetArtist(u.ID, id)
	if err != nil {
		return nil, fail(errNotFound, "艺术家不存在")
	}
	o := artistObj(*artist)
	o.Set("album", h.albumList(artist.Albums))
	return NewObj().Set("artist", o), nil
}

func (h *Handler) getAlbum(r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	album, err := h.Store.GetAlbum(u.ID, id, true)
	if err != nil {
		return nil, fail(errNotFound, "专辑不存在")
	}
	o := albumObj(*album)
	o.Set("song", h.songList(album.Songs))
	return NewObj().Set("album", o), nil
}

func (h *Handler) getSong(r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	song, err := h.Store.GetSong(u.ID, id)
	if err != nil {
		return nil, fail(errNotFound, "歌曲不存在")
	}
	return NewObj().Set("song", songObj(*song)), nil
}

func (h *Handler) getAlbumList(r *http.Request, u *store.User, method string) (*Obj, error) {
	kind := param(r, "type")
	size := util.Clamp(intParam(r, "size", 10), 1, 500)
	offset := intParam(r, "offset", 0)
	genre := param(r, "genre")
	fromYear := intParam(r, "fromYear", 0)
	toYear := intParam(r, "toYear", 0)

	f := store.AlbumFilter{Genre: genre, FromYear: fromYear, ToYear: toYear, Limit: size, Offset: offset}
	switch kind {
	case "newest":
		f.Sort = "newest"
	case "recent":
		f.Sort = "recent"
	case "random":
		f.Sort = "random"
	case "alphabeticalByName":
		f.Sort = "alphabetical"
	case "alphabeticalByArtist":
		f.Sort = "alphabeticalByArtist"
	case "starred":
		f.Sort = "starred"
		f.StarredOnly = true
	case "byYear":
		f.Sort = "byYear"
	case "byGenre":
		f.Sort = "alphabetical"
		if genre == "" {
			return nil, fail(errMissingParam, "byGenre 需要 genre 参数")
		}
	case "frequent":
		f.Sort = "frequent"
	default:
		f.Sort = "newest"
	}
	albums, err := h.Store.ListAlbums(u.ID, f)
	if err != nil {
		return nil, err
	}
	key := "albumList2"
	if method == "getAlbumList" {
		key = "albumList"
	}
	return NewObj().Set(key, NewObj().Set("album", h.albumList(albums))), nil
}

func (h *Handler) getRandomSongs(r *http.Request, u *store.User) (*Obj, error) {
	size := util.Clamp(intParam(r, "size", 10), 1, 500)
	songs, err := h.Store.ListSongs(u.ID, store.SongFilter{
		Genre: param(r, "genre"), Sort: "random", Limit: size,
	})
	if err != nil {
		return nil, err
	}
	return NewObj().Set("randomSongs", NewObj().Set("song", h.songList(songs))), nil
}

func (h *Handler) getSongsByGenre(r *http.Request, u *store.User) (*Obj, error) {
	genre := param(r, "genre")
	if genre == "" {
		return nil, fail(errMissingParam, "缺少参数 genre")
	}
	size := util.Clamp(intParam(r, "count", 50), 1, 500)
	offset := intParam(r, "offset", 0)
	songs, err := h.Store.ListSongs(u.ID, store.SongFilter{Genre: genre, Sort: "title", Limit: size, Offset: offset})
	if err != nil {
		return nil, err
	}
	return NewObj().Set("songsByGenre", NewObj().Set("song", h.songList(songs))), nil
}

func (h *Handler) getGenres(r *http.Request, u *store.User) (*Obj, error) {
	genres, err := h.Store.Genres()
	if err != nil {
		return nil, err
	}
	arr := make([]*Obj, 0, len(genres))
	for _, g := range genres {
		arr = append(arr, NewObj().Set("songCount", g.SongCount).Set("albumCount", g.AlbumCount).Set("value", g.Name))
	}
	return NewObj().Set("genres", NewObj().Set("genre", arr)), nil
}

func (h *Handler) search(r *http.Request, u *store.User, method string) (*Obj, error) {
	q := param(r, "query")
	artistCount := util.Clamp(intParam(r, "artistCount", 20), 0, 200)
	albumCount := util.Clamp(intParam(r, "albumCount", 20), 0, 200)
	songCount := util.Clamp(intParam(r, "songCount", 20), 0, 500)
	offset := intParam(r, "offset", 0)

	artists, _ := h.Store.SearchArtists(u.ID, q, artistCount)
	albums, _ := h.Store.SearchAlbums(u.ID, q, albumCount)
	songs, _ := h.Store.ListSongs(u.ID, store.SongFilter{Search: q, Sort: "title", Limit: songCount, Offset: offset})

	key := "searchResult3"
	if method == "search2" {
		key = "searchResult2"
	}
	res := NewObj()
	if len(artists) > 0 {
		res.Set("artist", h.artistList(artists))
	}
	if len(albums) > 0 {
		res.Set("album", h.albumList(albums))
	}
	if len(songs) > 0 {
		res.Set("song", h.songList(songs))
	}
	return NewObj().Set(key, res), nil
}

func (h *Handler) getStarred(r *http.Request, u *store.User, method string) (*Obj, error) {
	artists, _ := h.Store.StarredArtists(u.ID)
	albums, _ := h.Store.StarredAlbums(u.ID)
	songs, _ := h.Store.StarredSongs(u.ID, 500)
	key := "starred2"
	if method == "getStarred" {
		key = "starred"
	}
	o := NewObj()
	o.Set("artist", h.artistList(artists))
	o.Set("album", h.albumList(albums))
	o.Set("song", h.songList(songs))
	return NewObj().Set(key, o), nil
}

func (h *Handler) setStar(r *http.Request, u *store.User, star bool) (*Obj, error) {
	q := r.URL.Query()
	any := false
	for _, id := range q["id"] {
		if id == "" {
			continue
		}
		if err := h.Store.SetStar(u.ID, id, "song", star); err != nil {
			return nil, err
		}
		any = true
	}
	for _, id := range q["albumId"] {
		if id == "" {
			continue
		}
		if err := h.Store.SetStar(u.ID, id, "album", star); err != nil {
			return nil, err
		}
		any = true
	}
	for _, id := range q["artistId"] {
		if id == "" {
			continue
		}
		if err := h.Store.SetStar(u.ID, id, "artist", star); err != nil {
			return nil, err
		}
		any = true
	}
	if !any {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	return NewObj(), nil
}

func (h *Handler) setRating(r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	rating, err := strconv.Atoi(param(r, "rating"))
	if err != nil {
		return nil, fail(errMissingParam, "缺少参数 rating")
	}
	if err := h.Store.SetRating(u.ID, id, rating); err != nil {
		return nil, err
	}
	return NewObj(), nil
}

func (h *Handler) scrobble(r *http.Request, u *store.User) (*Obj, error) {
	ids := r.URL.Query()["id"]
	if len(ids) == 0 {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	at := int64(intParam(r, "time", 0)) / 1000
	if at <= 0 {
		at = nowUnix()
	}
	submission := true
	if v := param(r, "submission"); v == "false" || v == "0" {
		submission = false
	}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if submission {
			if err := h.Store.Scrobble(u.ID, id, at); err != nil {
				return nil, err
			}
		} else {
			_ = h.Store.TouchNowPlaying(u.ID, id, at)
		}
	}
	return NewObj(), nil
}

func (h *Handler) getUser(r *http.Request, u *store.User) (*Obj, error) {
	name := param(r, "username")
	if name == "" {
		name = u.Username
	}
	target, err := h.Store.GetUserByName(name)
	if err != nil {
		return nil, fail(errNotFound, "用户不存在")
	}
	if target.ID != u.ID && !u.IsAdmin {
		return nil, fail(errNotAuthorized, "没有权限查看其他用户")
	}
	o := NewObj()
	o.Set("username", target.Username)
	o.Set("email", "")
	o.Set("scrobblingEnabled", true)
	o.Set("adminRole", target.IsAdmin)
	o.Set("settingsRole", target.IsAdmin)
	o.Set("downloadRole", true)
	o.Set("uploadRole", target.IsAdmin)
	o.Set("playlistRole", true)
	o.Set("coverArtRole", true)
	o.Set("commentRole", false)
	o.Set("podcastRole", false)
	o.Set("streamRole", true)
	o.Set("jukeboxRole", false)
	o.Set("shareRole", false)
	o.Set("videoConversionRole", false)
	return NewObj().Set("user", o), nil
}

func (h *Handler) getUsers(r *http.Request, u *store.User) (*Obj, error) {
	if !u.IsAdmin {
		return nil, fail(errNotAuthorized, "需要管理员权限")
	}
	users, err := h.Store.ListUsers()
	if err != nil {
		return nil, err
	}
	arr := make([]*Obj, 0, len(users))
	for _, item := range users {
		arr = append(arr, NewObj().Set("username", item.Username).Set("adminRole", item.IsAdmin).
			Set("streamRole", true).Set("playlistRole", true).Set("coverArtRole", true).
			Set("downloadRole", true).Set("uploadRole", item.IsAdmin))
	}
	return NewObj().Set("users", NewObj().Set("user", arr)), nil
}

func (h *Handler) getScanStatus(r *http.Request, u *store.User) (*Obj, error) {
	st := h.Scanner.Status()
	count, _ := h.Store.CountSongs()
	o := NewObj().Set("scanning", st.Running).Set("count", count)
	if st.FinishedAt > 0 {
		o.Set("lastScan", isoTime(st.FinishedAt))
	}
	return NewObj().Set("scanStatus", o), nil
}

func (h *Handler) startScan(r *http.Request, u *store.User) (*Obj, error) {
	if !u.IsAdmin {
		return nil, fail(errNotAuthorized, "需要管理员权限")
	}
	// Never pass the HTTP request context here: it is cancelled as soon as this
	// response is written, which would abort the scan after the first file.
	h.Scanner.Start(context.Background())
	return h.getScanStatus(r, u)
}

func (h *Handler) getArtistInfo(r *http.Request, u *store.User, method string) (*Obj, error) {
	key := "artistInfo2"
	if method == "getArtistInfo" {
		key = "artistInfo"
	}
	return NewObj().Set(key, NewObj().
		Set("biography", "").
		Set("musicBrainzId", "").
		Set("lastFmUrl", "").
		Set("smallImageUrl", "").
		Set("mediumImageUrl", "").
		Set("largeImageUrl", "")), nil
}

func (h *Handler) getAlbumInfo(r *http.Request, u *store.User, method string) (*Obj, error) {
	key := "albumInfo2"
	if method == "getAlbumInfo" {
		key = "albumInfo"
	}
	return NewObj().Set(key, NewObj().
		Set("notes", "").
		Set("musicBrainzId", "").
		Set("lastFmUrl", "").
		Set("smallImageUrl", "").
		Set("mediumImageUrl", "").
		Set("largeImageUrl", "")), nil
}

func nowUnix() int64 { return timeNow() }

var _ = strings.TrimSpace

func timeNow() int64 { return timeNowUnix() }
