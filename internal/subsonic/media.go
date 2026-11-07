package subsonic

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mrome/internal/store"
	"mrome/internal/util"
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
	// http.ServeContent inside ServeMedia implements Range / If-Range / HEAD
	// semantics, including 206 Partial Content with a correct Content-Range.
	if err := util.ServeMedia(w, r, f.Path, f.ContentType, attachment); err != nil {
		return nil, fail(errNotFound, "音频文件不可用")
	}
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
	if err := util.ServeImage(w, r, path, mime, id); err != nil {
		return nil, fail(errNotFound, "封面不存在")
	}
	return nil, nil
}

func (h *Handler) resolveCover(id string) (string, string, bool) {
	p, m := h.Store.CoverPath(id)
	return p, m, p != ""
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
