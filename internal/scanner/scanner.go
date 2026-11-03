package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"

	"mrome/internal/media"
	"mrome/internal/store"
	"mrome/internal/util"
)

type State struct {
	Running    bool   `json:"running"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Added      int    `json:"added"`
	Updated    int    `json:"updated"`
	Removed    int    `json:"removed"`
	Failed     int    `json:"failed"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
	Message    string `json:"message"`
	Current    string `json:"current"`
}

type Scanner struct {
	store    *store.Store
	musicDir string
	covers   string
	trash    string

	mu    sync.Mutex
	state State
}

func New(st *store.Store, musicDir, coversDir, trashDir string) *Scanner {
	return &Scanner{store: st, musicDir: musicDir, covers: coversDir, trash: trashDir}
}

func (s *Scanner) Status() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Start kicks off a scan in the background. It flips the running flag before
// returning so that an immediately following status request reports the scan
// instead of a stale "idle" state.
func (s *Scanner) Start(ctx context.Context) bool {
	s.mu.Lock()
	if s.state.Running {
		s.mu.Unlock()
		return false
	}
	s.state = State{Running: true, StartedAt: time.Now().Unix(), Message: "正在遍历音乐目录"}
	s.mu.Unlock()
	go func() {
		_, _ = s.run(ctx)
	}()
	return true
}

// Scan walks the music directory and reconciles it with the database.
func (s *Scanner) Scan(ctx context.Context) (State, error) {
	s.mu.Lock()
	if s.state.Running {
		st := s.state
		s.mu.Unlock()
		return st, errors.New("扫描正在进行中")
	}
	s.state = State{Running: true, StartedAt: time.Now().Unix(), Message: "正在遍历音乐目录"}
	s.mu.Unlock()
	return s.run(ctx)
}

func (s *Scanner) run(ctx context.Context) (state State, err error) {
	// A partial directory walk must never feed DeleteMissing: an unreadable
	// folder would otherwise be treated as "all of its files were deleted".
	walkIncomplete := false
	defer func() {
		s.mu.Lock()
		s.state.Running = false
		s.state.FinishedAt = time.Now().Unix()
		s.state.Current = ""
		switch {
		case ctx.Err() != nil:
			s.state.Message = "扫描已中断"
		case err != nil:
			s.state.Message = "扫描失败: " + err.Error()
		case walkIncomplete:
			s.state.Message = "扫描完成（部分目录不可读，已保留原有曲目）"
		default:
			s.state.Message = "扫描完成"
		}
		s.mu.Unlock()
	}()
	// A malformed file must not take the whole server down; convert the panic
	// into a failed scan so the UI can report it. Registered last, so it runs
	// before the state-reset defer above and can hand it the error.
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("扫描异常: %v", rec)
		}
	}()

	type fileEntry struct{ abs, rel string }
	var files []fileEntry
	err = filepath.WalkDir(s.musicDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			walkIncomplete = true
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "@eaDir" || name == "#recycle" {
				if path != s.musicDir {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !media.IsAudio(path) {
			return nil
		}
		rel, err := filepath.Rel(s.musicDir, path)
		if err != nil {
			return nil
		}
		files = append(files, fileEntry{abs: path, rel: rel})
		return nil
	})
	if err != nil {
		return s.Status(), err
	}
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].rel) < strings.ToLower(files[j].rel) })

	s.mu.Lock()
	s.state.Total = len(files)
	s.state.Message = "正在解析元数据"
	s.mu.Unlock()

	seen := make(map[string]bool, len(files))
	folderCovers := map[string]folderCoverResult{}
	for i, f := range files {
		select {
		case <-ctx.Done():
			return s.Status(), ctx.Err()
		default:
		}
		// The song ID is derived from the relative path, so the row must survive
		// this scan even if reading the file fails transiently.
		seen[store.SongID(f.rel)] = true
		id, added, err := s.ingest(f.abs, f.rel, folderCovers, "")
		s.mu.Lock()
		s.state.Done = i + 1
		s.state.Current = f.rel
		if err != nil {
			s.state.Failed++
		} else {
			seen[id] = true
			if added {
				s.state.Added++
			} else {
				s.state.Updated++
			}
		}
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.state.Message = "正在整理专辑信息"
	s.mu.Unlock()

	removed := 0
	if !walkIncomplete {
		removed, _ = s.store.DeleteMissing(seen)
	}
	deduped := s.dedupByContent()
	if err := s.store.RefreshAlbumStats(); err != nil {
		return s.Status(), err
	}

	s.mu.Lock()
	s.state.Removed = removed + deduped
	if deduped > 0 {
		s.state.Message = fmt.Sprintf("扫描完成，按内容去重清理 %d 首重复歌曲", deduped)
	}
	s.mu.Unlock()
	return s.Status(), nil
}

// dedupByContent removes songs whose file bytes are identical to an earlier
// import: the earliest copy stays, the rest are moved to the trash so nothing
// is destroyed irreversibly. Files without a hash (not yet backfilled) are
// left alone.
func (s *Scanner) dedupByContent() int {
	groups, err := s.store.MD5Duplicates()
	if err != nil {
		return 0
	}
	n := 0
	for _, g := range groups {
		for _, id := range g.Extra {
			if p, err := s.store.SongPath(id); err == nil && p != "" {
				s.trashFile(p)
			}
			if err := s.store.DeleteSong(id); err == nil {
				n++
			}
		}
	}
	return n
}

// trashFile moves a library file into the trash dir, mirroring the manual
// delete flow so a bad dedup can be undone by hand.
func (s *Scanner) trashFile(p string) {
	if s.trash == "" {
		return
	}
	if err := os.MkdirAll(s.trash, 0o755); err != nil {
		return
	}
	target := util.UniquePath(filepath.Join(s.trash, filepath.Base(p)))
	if err := os.Rename(p, target); err != nil {
		// Cross-device fallback: copy then remove.
		if in, ierr := os.Open(p); ierr == nil {
			out, oerr := os.Create(target)
			if oerr == nil {
				_, _ = io.Copy(out, in)
				out.Close()
			}
			in.Close()
			_ = os.Remove(p)
		}
	}
}

// ScanFile ingests a single uploaded file.
func (s *Scanner) ScanFile(absPath string) (string, error) {
	return s.ScanFileHashed(absPath, "")
}

// ScanFileHashed ingests a single file whose content hash is already known
// (upload completion), saving a second read of the file.
func (s *Scanner) ScanFileHashed(absPath, md5 string) (string, error) {
	rel, err := filepath.Rel(s.musicDir, absPath)
	if err != nil {
		rel = filepath.Base(absPath)
	}
	id, added, err := s.ingest(absPath, rel, map[string]folderCoverResult{}, md5)
	if err != nil {
		return "", err
	}
	if err := s.store.RefreshAlbumStats(); err != nil {
		return id, err
	}
	// Keep the last-scan summary honest when a single file is ingested
	// outside a full scan (upload completion).
	s.mu.Lock()
	if !s.state.Running {
		s.state.Total++
		s.state.Done++
		if added {
			s.state.Added++
		} else {
			s.state.Updated++
		}
		if s.state.Message == "" || s.state.Message == "正在遍历音乐目录" {
			s.state.Message = "扫描完成"
		}
	}
	s.mu.Unlock()
	return id, nil
}

func (s *Scanner) ingest(abs, rel string, folderCovers map[string]folderCoverResult, knownMD5 string) (string, bool, error) {
	if full, err := filepath.Abs(abs); err == nil {
		abs = full
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", false, err
	}
	in := store.TrackInput{
		RelPath: rel,
		AbsPath: abs,
		Size:    st.Size(),
		ModTime: st.ModTime().Unix(),
		Suffix:  strings.TrimPrefix(strings.ToLower(filepath.Ext(abs)), "."),
	}
	if ct, ok := media.ContentType(abs); ok {
		in.ContentType = ct
	}

	var embeddedLyrics string
	f, err := os.Open(abs)
	if err == nil {
		if m, terr := tag.ReadFrom(f); terr == nil {
			in.Title = util.CleanName(m.Title())
			in.Artist = util.CleanName(m.Artist())
			in.Album = util.CleanName(m.Album())
			in.AlbumArtist = util.CleanName(m.AlbumArtist())
			in.Genre = util.CleanName(m.Genre())
			in.Year = m.Year()
			in.Track, _ = m.Track()
			in.Disc, _ = m.Disc()
			embeddedLyrics = m.Lyrics()
			if pic := m.Picture(); pic != nil && len(pic.Data) > 0 && len(pic.Data) < 12<<20 {
				in.CoverData = pic.Data
				in.CoverMime = pic.MIMEType
				if in.CoverMime == "" {
					in.CoverMime = "image/jpeg"
				}
			}
		}
		f.Close()
	}

	if in.Title == "" {
		in.Title = strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	}
	if in.Artist == "" {
		in.Artist = "未知艺术家"
	}
	if in.Album == "" {
		in.Album = "未知专辑"
	}
	if in.AlbumArtist == "" {
		in.AlbumArtist = in.Artist
	}

	info := media.Probe(abs)
	in.Duration = info.Duration
	in.Bitrate = info.Bitrate
	in.SampleRate = info.SampleRate
	in.Channels = info.Channels

	// Sidecar lyrics (.lrc) take precedence over embedded ones.
	base := strings.TrimSuffix(abs, filepath.Ext(abs))
	for _, ext := range []string{".lrc", ".LRC"} {
		if b, err := os.ReadFile(base + ext); err == nil && len(b) < 1<<20 {
			in.Lyrics = string(b)
			break
		}
	}
	if in.Lyrics == "" {
		in.Lyrics = embeddedLyrics
	}

	// Content hash for duplicate detection. Unchanged files reuse the stored
	// hash; everything else (new, changed, or not yet hashed) is read once.
	if knownMD5 != "" {
		in.MD5 = knownMD5
	} else if old, size, mtime, ok := s.store.StoredFileInfo(abs); ok && old != "" &&
		size == st.Size() && mtime == st.ModTime().Unix() {
		in.MD5 = old
	} else if h, herr := util.MD5File(abs); herr == nil {
		in.MD5 = h
	}

	// Folder-level cover art when the file carries none. The lookup is cached
	// per directory for the duration of a scan so a 20-track album reads its
	// cover once, not twenty times.
	if len(in.CoverData) == 0 {
		if data, mime, ok := folderCoverCached(filepath.Dir(abs), folderCovers); ok {
			in.CoverData = data
			in.CoverMime = mime
		}
	}

	added := !s.store.PathExists(filepath.ToSlash(abs))
	id, err := s.store.UpsertTrack(in)
	if err != nil {
		return "", false, err
	}
	return id, added, nil
}

type folderCoverResult struct {
	data []byte
	mime string
	ok   bool
}

func folderCoverCached(dir string, cache map[string]folderCoverResult) ([]byte, string, bool) {
	if r, ok := cache[dir]; ok {
		return r.data, r.mime, r.ok
	}
	data, mime, ok := folderCover(dir)
	cache[dir] = folderCoverResult{data: data, mime: mime, ok: ok}
	return data, mime, ok
}

func folderCover(dir string) ([]byte, string, bool) {
	names := []string{"cover.jpg", "cover.jpeg", "cover.png", "folder.jpg", "folder.png", "front.jpg", "album.jpg", "AlbumArt.jpg"}
	for _, n := range names {
		p := filepath.Join(dir, n)
		if st, err := os.Stat(p); err == nil && st.Size() < 12<<20 {
			b, err := os.ReadFile(p)
			if err == nil {
				mime := "image/jpeg"
				if strings.HasSuffix(strings.ToLower(n), ".png") {
					mime = "image/png"
				}
				return b, mime, true
			}
		}
	}
	return nil, "", false
}
