package subsonic

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mrome/internal/store"
)

func (h *Handler) stream(w http.ResponseWriter, r *http.Request, u *store.User) (*Obj, error) {
	return h.serveAudio(w, r, false)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request, u *store.User) (*Obj, error) {
	return h.serveAudio(w, r, true)
}

func (h *Handler) serveAudio(w http.ResponseWriter, r *http.Request, attachment bool) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	f, err := h.Store.GetSongFile(id)
	if err != nil {
		return nil, fail(errNotFound, "歌曲不存在")
	}
	file, err := os.Open(f.Path)
	if err != nil {
		return nil, fail(errNotFound, "音频文件不可用")
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		return nil, fail(errNotFound, "音频文件不可用")
	}

	ct := f.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-cache")
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, baseName(f.Path)))
	// http.ServeContent implements Range / If-Range / HEAD semantics, including
	// 206 Partial Content with a correct Content-Range header.
	http.ServeContent(w, r, baseName(f.Path), st.ModTime(), file)
	return nil, nil
}

func (h *Handler) getCoverArt(w http.ResponseWriter, r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	path, mime, ok := h.resolveCover(id)
	if !ok {
		return nil, fail(errNotFound, "封面不存在")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fail(errNotFound, "封面不存在")
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		return nil, fail(errNotFound, "封面不存在")
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Header().Set("ETag", `"`+id+`-`+strconv.FormatInt(st.ModTime().Unix(), 10)+`"`)
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), file)
	return nil, nil
}

func (h *Handler) resolveCover(id string) (string, string, bool) {
	if p, m, err := h.Store.AlbumCoverPath(id); err == nil && p != "" {
		if _, statErr := os.Stat(p); statErr == nil {
			return p, m, true
		}
	}
	if s, err := h.Store.GetSong("", id); err == nil && s.AlbumID != "" {
		if p, m, err := h.Store.AlbumCoverPath(s.AlbumID); err == nil && p != "" {
			if _, statErr := os.Stat(p); statErr == nil {
				return p, m, true
			}
		}
	}
	if p, m, err := h.Store.ArtistCover(id); err == nil && p != "" {
		if _, statErr := os.Stat(p); statErr == nil {
			return p, m, true
		}
	}
	return "", "", false
}

var lrcTag = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)

func parseLRC(text string) []*Obj {
	type line struct {
		start int
		text  string
	}
	var lines []line
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimRight(raw, "\r")
		matches := lrcTag.FindAllStringSubmatch(raw, -1)
		if len(matches) == 0 {
			continue
		}
		value := strings.TrimSpace(lrcTag.ReplaceAllString(raw, ""))
		for _, m := range matches {
			min, _ := strconv.Atoi(m[1])
			sec, _ := strconv.Atoi(m[2])
			ms := 0
			if m[3] != "" {
				frac := m[3]
				switch len(frac) {
				case 1:
					frac += "00"
				case 2:
					frac += "0"
				}
				ms, _ = strconv.Atoi(frac)
			}
			lines = append(lines, line{start: min*60000 + sec*1000 + ms, text: value})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].start < lines[j].start })
	out := make([]*Obj, 0, len(lines))
	for _, l := range lines {
		out = append(out, NewObj().Set("start", l.start).Set("value", l.text))
	}
	return out
}

func (h *Handler) getLyricsBySongID(r *http.Request, u *store.User) (*Obj, error) {
	id := param(r, "id")
	if id == "" {
		return nil, fail(errMissingParam, "缺少参数 id")
	}
	song, err := h.Store.GetSong(u.ID, id)
	if err != nil {
		return nil, fail(errNotFound, "歌曲不存在")
	}
	raw, _ := h.Store.SongLyrics(id)
	if strings.TrimSpace(raw) == "" {
		return NewObj().Set("lyricsList", NewObj().Set("lyrics", []*Obj{})), nil
	}
	lines := parseLRC(raw)
	lyric := NewObj().
		Set("displayArtist", song.Artist).
		Set("displayTitle", song.Title).
		Set("synced", len(lines) > 0).
		Set("value", raw)
	if len(lines) > 0 {
		lyric.Set("line", lines)
	}
	return NewObj().Set("lyricsList", NewObj().Set("lyrics", []*Obj{lyric})), nil
}

func (h *Handler) getLyrics(r *http.Request, u *store.User) (*Obj, error) {
	title := param(r, "title")
	artist := param(r, "artist")
	if title == "" {
		return NewObj().Set("lyrics", NewObj()), nil
	}
	songs, err := h.Store.ListSongs(u.ID, store.SongFilter{Search: title, Sort: "title", Limit: 50})
	if err != nil || len(songs) == 0 {
		return NewObj().Set("lyrics", NewObj()), nil
	}
	// Prefer an exact title + artist match, then an exact title match, then any
	// title match by the requested artist. Among equally good candidates, pick
	// one that actually has lyrics stored.
	best, bestScore := -1, -1
	for i := range songs {
		score := 0
		if strings.EqualFold(strings.TrimSpace(songs[i].Title), title) {
			score += 2
		}
		if artist != "" && strings.EqualFold(strings.TrimSpace(songs[i].Artist), artist) {
			score += 2
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 {
		best = 0
	}
	raw, _ := h.Store.SongLyrics(songs[best].ID)
	if strings.TrimSpace(raw) == "" {
		for i := range songs {
			if i == best {
				continue
			}
			if l, err := h.Store.SongLyrics(songs[i].ID); err == nil && strings.TrimSpace(l) != "" {
				best, raw = i, l
				break
			}
		}
	}
	pick := &songs[best]
	return NewObj().Set("lyrics", NewObj().
		Set("artist", pick.Artist).
		Set("title", pick.Title).
		Set("value", raw)), nil
}

func modTime(t time.Time) time.Time { return t }
