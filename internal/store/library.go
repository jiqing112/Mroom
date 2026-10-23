package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ArtistID(name string) string {
	return hashID("artist", strings.ToLower(strings.TrimSpace(name)))
}

func AlbumID(albumArtist, album string) string {
	return hashID("album", strings.ToLower(strings.TrimSpace(albumArtist)), strings.ToLower(strings.TrimSpace(album)))
}

func SongID(relPath string) string {
	return hashID("song", strings.ToLower(filepath.ToSlash(relPath)))
}

func (s *Store) SetCoversDir(dir string) { s.dir = dir }

const songCols = `s.id, s.title, s.artist_id, s.artist, s.album_id, s.album, s.album_artist,
	s.genre, s.track, s.disc, s.year, s.duration, s.bitrate, s.size, s.suffix, s.content_type,
	s.created_at, CASE WHEN s.lyrics <> '' THEN 1 ELSE 0 END,
	COALESCE(an.starred,0), COALESCE(an.starred_at,0), COALESCE(an.rating,0), COALESCE(an.play_count,0)`

const songJoin = ` FROM songs s LEFT JOIN annotations an
	ON an.item_id = s.id AND an.item_type = 'song' AND an.user_id = ?`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSong(r rowScanner) (Song, error) {
	var s Song
	err := r.Scan(&s.ID, &s.Title, &s.ArtistID, &s.Artist, &s.AlbumID, &s.Album, &s.AlbumArtist,
		&s.Genre, &s.Track, &s.Disc, &s.Year, &s.Duration, &s.Bitrate, &s.Size, &s.Suffix, &s.ContentType,
		&s.Created, &s.HasLyrics, &s.Starred, &s.StarredAt, &s.Rating, &s.PlayCount)
	if err != nil {
		return s, err
	}
	s.CoverArt = s.AlbumID
	return s, nil
}

func (s *Store) ListSongs(userID string, f SongFilter) ([]Song, error) {
	var where []string
	var args []any
	args = append(args, userID)
	if f.Search != "" {
		where = append(where, `(s.title LIKE ? OR s.artist LIKE ? OR s.album LIKE ?)`)
		l := like(f.Search)
		args = append(args, l, l, l)
	}
	if f.AlbumID != "" {
		where = append(where, `s.album_id = ?`)
		args = append(args, f.AlbumID)
	}
	if f.ArtistID != "" {
		where = append(where, `(s.artist_id = ? OR s.album_id IN (SELECT id FROM albums WHERE artist_id = ?))`)
		args = append(args, f.ArtistID, f.ArtistID)
	}
	if f.Genre != "" {
		where = append(where, `s.genre = ?`)
		args = append(args, f.Genre)
	}
	if f.StarredOnly {
		where = append(where, `COALESCE(an.starred,0) = 1`)
	}
	q := `SELECT ` + songCols + songJoin
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	switch f.Sort {
	case "artist":
		q += ` ORDER BY s.artist COLLATE NOCASE, s.album COLLATE NOCASE, s.disc, s.track, s.title`
	case "album":
		q += ` ORDER BY s.album COLLATE NOCASE, s.disc, s.track, s.title`
	case "created", "newest":
		q += ` ORDER BY s.created_at DESC, s.title`
	case "random":
		q += ` ORDER BY RANDOM()`
	default:
		q += ` ORDER BY s.title COLLATE NOCASE`
	}
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT %d OFFSET %d`, f.Limit, f.Offset)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Song{}
	for rows.Next() {
		song, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, song)
	}
	return out, rows.Err()
}

func (s *Store) CountSongsFiltered(userID string, f SongFilter) (int, error) {
	var where []string
	var args []any
	args = append(args, userID)
	if f.Search != "" {
		l := like(f.Search)
		where = append(where, `(s.title LIKE ? OR s.artist LIKE ? OR s.album LIKE ?)`)
		args = append(args, l, l, l)
	}
	if f.AlbumID != "" {
		where = append(where, `s.album_id = ?`)
		args = append(args, f.AlbumID)
	}
	if f.ArtistID != "" {
		where = append(where, `(s.artist_id = ? OR s.album_id IN (SELECT id FROM albums WHERE artist_id = ?))`)
		args = append(args, f.ArtistID, f.ArtistID)
	}
	if f.Genre != "" {
		where = append(where, `s.genre = ?`)
		args = append(args, f.Genre)
	}
	if f.StarredOnly {
		where = append(where, `COALESCE(an.starred,0) = 1`)
	}
	q := `SELECT COUNT(*)` + songJoin
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	var n int
	err := s.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

func (s *Store) GetSong(userID, id string) (*Song, error) {
	row := s.db.QueryRow(`SELECT `+songCols+songJoin+` WHERE s.id = ?`, userID, id)
	song, err := scanSong(row)
	if err != nil {
		return nil, err
	}
	return &song, nil
}

// SongFile resolves everything needed to serve an audio stream.
type SongFile struct {
	ID          string
	Path        string
	Title       string
	Artist      string
	Suffix      string
	ContentType string
	Size        int64
	Created     int64
}

func (s *Store) GetSongFile(id string) (*SongFile, error) {
	var f SongFile
	err := s.db.QueryRow(`SELECT id, path, title, artist, suffix, content_type, size, created_at
		FROM songs WHERE id = ?`, id).Scan(&f.ID, &f.Path, &f.Title, &f.Artist, &f.Suffix, &f.ContentType, &f.Size, &f.Created)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *Store) SongLyrics(id string) (string, error) {
	var l string
	err := s.db.QueryRow(`SELECT lyrics FROM songs WHERE id=?`, id).Scan(&l)
	return l, err
}

func (s *Store) UpdateSongMeta(id, title, artist, albumArtist, album, genre string, year, track int) error {
	existing, err := s.GetSong("", id)
	if err != nil {
		return err
	}
	artist = strings.TrimSpace(artist)
	album = strings.TrimSpace(album)
	albumArtist = strings.TrimSpace(albumArtist)
	if artist == "" {
		artist = "未知艺术家"
	}
	if album == "" {
		album = "未知专辑"
	}
	// Preserve the album's own artist unless the caller explicitly changes it;
	// otherwise editing one track of a compilation would split it into a new
	// album named after the track artist.
	if albumArtist == "" {
		albumArtist = existing.AlbumArtist
	}
	if albumArtist == "" {
		albumArtist = artist
	}
	aid := ArtistID(artist)
	albumArtistID := ArtistID(albumArtist)
	alid := AlbumID(albumArtist, album)
	ts := now()
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO artists(id,name,created_at) VALUES(?,?,?)`, aid, artist, ts); err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO artists(id,name,created_at) VALUES(?,?,?)`, albumArtistID, albumArtist, ts); err != nil {
		return err
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO albums(id,name,artist_id,artist,year,genre,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		alid, album, albumArtistID, albumArtist, year, genre, ts, ts); err != nil {
		return err
	}
	// Moving a track to another album should carry its artwork along unless the
	// destination already has its own cover.
	if existing.AlbumID != "" && existing.AlbumID != alid && s.dir != "" {
		var oldPath, oldMime, newPath string
		_ = s.db.QueryRow(`SELECT cover_path, cover_mime FROM albums WHERE id=?`, existing.AlbumID).Scan(&oldPath, &oldMime)
		_ = s.db.QueryRow(`SELECT cover_path FROM albums WHERE id=?`, alid).Scan(&newPath)
		if oldPath != "" && newPath == "" {
			if data, err := os.ReadFile(oldPath); err == nil {
				ext := filepath.Ext(oldPath)
				if ext == "" {
					ext = coverExt(oldMime)
				}
				target := filepath.Join(s.dir, alid+ext)
				if err := os.WriteFile(target, data, 0o644); err == nil {
					_, _ = s.db.Exec(`UPDATE albums SET cover_path=?, cover_mime=? WHERE id=?`, target, oldMime, alid)
				}
			}
		}
	}
	_, err = s.db.Exec(`UPDATE songs SET title=?, artist=?, artist_id=?, album=?, album_id=?, album_artist=?,
			genre=?, year=?, track=?, meta_edited=1, updated_at=? WHERE id=?`,
		title, artist, aid, album, alid, albumArtist, genre, year, track, ts, id)
	if err != nil {
		return err
	}
	// Record the current on-disk mtime/size so the next scan recognises that
	// the file itself is unchanged and keeps the edit.
	if st, statErr := os.Stat(existing.Path); statErr == nil {
		_, _ = s.db.Exec(`UPDATE songs SET file_mtime=?, size=? WHERE id=?`, st.ModTime().Unix(), st.Size(), id)
	}
	return s.RefreshAlbumStats()
}

func (s *Store) RefreshAlbumStats() error {
	if _, err := s.db.Exec(`UPDATE albums SET
		song_count = (SELECT COUNT(*) FROM songs WHERE album_id = albums.id),
		duration   = (SELECT CAST(COALESCE(SUM(duration),0) AS INTEGER) FROM songs WHERE album_id = albums.id),
		updated_at = ?
		WHERE EXISTS (SELECT 1 FROM songs WHERE album_id = albums.id)`, now()); err != nil {
		return err
	}
	return s.PruneEmpty()
}

// ---------------------------------------------------------------- artists

func (s *Store) ListArtists(userID string) ([]Artist, error) {
	rows, err := s.db.Query(`
		SELECT ar.id, ar.name,
		  (SELECT COUNT(*) FROM albums WHERE artist_id = ar.id) AS album_count,
		  (SELECT COUNT(*) FROM songs WHERE artist_id = ar.id OR album_id IN (SELECT id FROM albums WHERE artist_id = ar.id)) AS song_count,
		  COALESCE((SELECT id FROM albums WHERE artist_id = ar.id AND cover_path<>'' ORDER BY year DESC, created_at DESC LIMIT 1),'') AS cover,
		  COALESCE((SELECT starred FROM annotations WHERE user_id=? AND item_id=ar.id AND item_type='artist'),0),
		  COALESCE((SELECT starred_at FROM annotations WHERE user_id=? AND item_id=ar.id AND item_type='artist'),0)
		FROM artists ar
		WHERE EXISTS (SELECT 1 FROM albums WHERE artist_id = ar.id)
		   OR EXISTS (SELECT 1 FROM songs WHERE artist_id = ar.id)
		ORDER BY ar.name COLLATE NOCASE`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Artist{}
	for rows.Next() {
		var a Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.SongCount, &a.CoverArt, &a.Starred, &a.StarredAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetArtist(userID, id string) (*Artist, error) {
	var a Artist
	err := s.db.QueryRow(`SELECT ar.id, ar.name,
		(SELECT COUNT(*) FROM albums WHERE artist_id=ar.id),
		(SELECT COUNT(*) FROM songs WHERE artist_id=ar.id OR album_id IN (SELECT id FROM albums WHERE artist_id=ar.id)),
		COALESCE((SELECT id FROM albums WHERE artist_id=ar.id AND cover_path<>'' ORDER BY year DESC, created_at DESC LIMIT 1),''),
		COALESCE((SELECT starred FROM annotations WHERE user_id=? AND item_id=ar.id AND item_type='artist'),0),
		COALESCE((SELECT starred_at FROM annotations WHERE user_id=? AND item_id=ar.id AND item_type='artist'),0)
		FROM artists ar WHERE ar.id=?`, userID, userID, id).Scan(&a.ID, &a.Name, &a.AlbumCount, &a.SongCount, &a.CoverArt, &a.Starred, &a.StarredAt)
	if err != nil {
		return nil, err
	}
	albums, err := s.ListAlbums(userID, AlbumFilter{ArtistID: id})
	if err != nil {
		return nil, err
	}
	a.Albums = albums
	return &a, nil
}

// ---------------------------------------------------------------- albums

type AlbumFilter struct {
	Search      string
	Sort        string
	Genre       string
	ArtistID    string
	FromYear    int
	ToYear      int
	StarredOnly bool
	Limit       int
	Offset      int
}

func (s *Store) ListAlbums(userID string, f AlbumFilter) ([]Album, error) {
	var where []string
	var args []any
	args = append(args, userID)
	if f.Search != "" {
		where = append(where, `(al.name LIKE ? OR al.artist LIKE ?)`)
		l := like(f.Search)
		args = append(args, l, l)
	}
	if f.Genre != "" {
		where = append(where, `al.genre = ?`)
		args = append(args, f.Genre)
	}
	if f.ArtistID != "" {
		where = append(where, `al.artist_id = ?`)
		args = append(args, f.ArtistID)
	}
	if f.FromYear > 0 {
		where = append(where, `al.year >= ?`)
		args = append(args, f.FromYear)
	}
	if f.ToYear > 0 {
		where = append(where, `al.year <= ?`)
		args = append(args, f.ToYear)
	}
	if f.StarredOnly {
		where = append(where, `COALESCE(an.starred,0)=1`)
	}
	q := `SELECT al.id, al.name, al.artist_id, al.artist, al.year, al.genre,
			al.song_count, al.duration, al.created_at,
			COALESCE(an.starred,0), COALESCE(an.starred_at,0), COALESCE(an.rating,0)
		FROM albums al
		LEFT JOIN annotations an ON an.item_id=al.id AND an.item_type='album' AND an.user_id=?`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	switch f.Sort {
	case "newest", "":
		q += ` ORDER BY al.created_at DESC, al.name`
	case "recent":
		q += ` ORDER BY COALESCE((SELECT MAX(play_date) FROM annotations WHERE item_id=al.id AND item_type='album' AND user_id=?),0) DESC, al.created_at DESC`
		args = append(args, userID)
	case "alphabetical", "alphabeticalByName":
		q += ` ORDER BY al.name COLLATE NOCASE`
	case "alphabeticalByArtist":
		q += ` ORDER BY al.artist COLLATE NOCASE, al.year`
	case "byYear":
		q += ` ORDER BY al.year DESC, al.name`
	case "random":
		q += ` ORDER BY RANDOM()`
	case "starred":
		q += ` ORDER BY COALESCE(an.starred_at,0) DESC`
	case "frequent":
		q += ` ORDER BY COALESCE((SELECT SUM(play_count) FROM annotations WHERE item_id=al.id AND item_type='album' AND user_id=?),0) DESC`
		args = append(args, userID)
	default:
		q += ` ORDER BY al.created_at DESC`
	}
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT %d OFFSET %d`, f.Limit, f.Offset)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Album{}
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.ID, &a.Name, &a.ArtistID, &a.Artist, &a.Year, &a.Genre,
			&a.SongCount, &a.Duration, &a.Created, &a.Starred, &a.StarredAt, &a.Rating); err != nil {
			return nil, err
		}
		a.CoverArt = a.ID
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAlbum(userID, id string, withSongs bool) (*Album, error) {
	var a Album
	err := s.db.QueryRow(`SELECT al.id, al.name, al.artist_id, al.artist, al.year, al.genre,
			al.song_count, al.duration, al.created_at,
			COALESCE(an.starred,0), COALESCE(an.starred_at,0), COALESCE(an.rating,0)
		FROM albums al
		LEFT JOIN annotations an ON an.item_id=al.id AND an.item_type='album' AND an.user_id=?
		WHERE al.id=?`, userID, id).Scan(&a.ID, &a.Name, &a.ArtistID, &a.Artist, &a.Year, &a.Genre,
		&a.SongCount, &a.Duration, &a.Created, &a.Starred, &a.StarredAt, &a.Rating)
	if err != nil {
		return nil, err
	}
	a.CoverArt = a.ID
	if withSongs {
		songs, err := s.ListSongs(userID, SongFilter{AlbumID: id, Sort: "album"})
		if err != nil {
			return nil, err
		}
		a.Songs = songs
	}
	return &a, nil
}

func (s *Store) AlbumCoverPath(id string) (path, mime string, err error) {
	err = s.db.QueryRow(`SELECT cover_path, cover_mime FROM albums WHERE id=?`, id).Scan(&path, &mime)
	return
}

// ---------------------------------------------------------------- genres

func (s *Store) Genres() ([]Genre, error) {
	rows, err := s.db.Query(`SELECT genre, COUNT(*), COUNT(DISTINCT album_id) FROM songs
		WHERE genre <> '' GROUP BY genre ORDER BY genre COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Genre{}
	for rows.Next() {
		var g Genre
		if err := rows.Scan(&g.Name, &g.SongCount, &g.AlbumCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- search

func (s *Store) SearchArtists(userID, q string, limit int) ([]Artist, error) {
	rows, err := s.db.Query(`SELECT ar.id, ar.name,
		(SELECT COUNT(*) FROM albums WHERE artist_id=ar.id),
		(SELECT COUNT(*) FROM songs WHERE artist_id=ar.id),
		COALESCE((SELECT id FROM albums WHERE artist_id=ar.id AND cover_path<>'' ORDER BY year DESC, created_at DESC LIMIT 1),''),
		COALESCE((SELECT starred FROM annotations WHERE user_id=? AND item_id=ar.id AND item_type='artist'),0),
		COALESCE((SELECT starred_at FROM annotations WHERE user_id=? AND item_id=ar.id AND item_type='artist'),0)
		FROM artists ar WHERE ar.name LIKE ? ORDER BY ar.name LIMIT ?`, userID, userID, like(q), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Artist{}
	for rows.Next() {
		var a Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.SongCount, &a.CoverArt, &a.Starred, &a.StarredAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) SearchAlbums(userID, q string, limit int) ([]Album, error) {
	return s.ListAlbums(userID, AlbumFilter{Search: q, Sort: "alphabetical", Limit: limit})
}

// ---------------------------------------------------------------- annotations

func (s *Store) SetStar(userID, itemID, itemType string, star bool) error {
	if userID == "" || itemID == "" {
		return fmt.Errorf("missing user or item")
	}
	if star {
		_, err := s.db.Exec(`INSERT INTO annotations(user_id,item_id,item_type,starred,starred_at)
			VALUES(?,?,?,1,?)
			ON CONFLICT(user_id,item_id) DO UPDATE SET starred=1, starred_at=excluded.starred_at`,
			userID, itemID, itemType, now())
		return err
	}
	_, err := s.db.Exec(`UPDATE annotations SET starred=0, starred_at=0 WHERE user_id=? AND item_id=?`, userID, itemID)
	return err
}

func (s *Store) SetRating(userID, itemID string, rating int) error {
	if rating < 0 {
		rating = 0
	}
	if rating > 5 {
		rating = 5
	}
	_, err := s.db.Exec(`INSERT INTO annotations(user_id,item_id,item_type,rating) VALUES(?,?,'song',?)
		ON CONFLICT(user_id,item_id) DO UPDATE SET rating=excluded.rating`, userID, itemID, rating)
	return err
}

func (s *Store) Scrobble(userID, songID string, at int64) error {
	if at <= 0 {
		at = now()
	}
	// The web player used to send Date.now() (milliseconds) while every client
	// protocol sends seconds. Normalise on write so ordering stays correct.
	if at > 1_000_000_000_000 {
		at = at / 1000
	}
	_, err := s.db.Exec(`INSERT INTO annotations(user_id,item_id,item_type,play_count,play_date)
		VALUES(?,?,'song',1,?)
		ON CONFLICT(user_id,item_id) DO UPDATE SET play_count=play_count+1, play_date=excluded.play_date`,
		userID, songID, at)
	if err != nil {
		return err
	}
	// Roll the play up to the album so "recently played" works in album lists.
	_, err = s.db.Exec(`INSERT INTO annotations(user_id,item_id,item_type,play_count,play_date)
		SELECT ?, album_id, 'album', 1, ? FROM songs WHERE id=? AND album_id<>''
		ON CONFLICT(user_id,item_id) DO UPDATE SET play_count=play_count+1, play_date=excluded.play_date`,
		userID, at, songID)
	return err
}

func (s *Store) StarredSongs(userID string, limit int) ([]Song, error) {
	return s.ListSongs(userID, SongFilter{StarredOnly: true, Sort: "created", Limit: limit})
}

// RecentlyPlayed returns songs ordered by the last time they were scrobbled.
// Both the web player and every client protocol write into annotations.play_date,
// so this works no matter where the playback happened.
func (s *Store) RecentlyPlayed(userID string, limit int) ([]Song, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	// Legacy rows may hold milliseconds; compare on a normalised value so old and
	// new play dates sort together.
	q := `SELECT ` + songCols + songJoin +
		` WHERE COALESCE(an.play_date,0) > 0
		ORDER BY (CASE WHEN an.play_date > 100000000000 THEN an.play_date/1000 ELSE an.play_date END) DESC
		LIMIT ?`
	rows, err := s.db.Query(q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Song{}
	for rows.Next() {
		song, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, song)
	}
	return out, rows.Err()
}

func (s *Store) StarredAlbums(userID string) ([]Album, error) {
	return s.ListAlbums(userID, AlbumFilter{StarredOnly: true, Sort: "starred"})
}

func (s *Store) StarredArtists(userID string) ([]Artist, error) {
	rows, err := s.db.Query(`SELECT ar.id, ar.name,
		(SELECT COUNT(*) FROM albums WHERE artist_id=ar.id),
		(SELECT COUNT(*) FROM songs WHERE artist_id=ar.id),
		COALESCE((SELECT id FROM albums WHERE artist_id=ar.id AND cover_path<>'' ORDER BY year DESC, created_at DESC LIMIT 1),''), 1, COALESCE(an.starred_at,0)
		FROM artists ar JOIN annotations an ON an.item_id=ar.id AND an.item_type='artist'
		WHERE an.user_id=? AND an.starred=1 ORDER BY an.starred_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Artist{}
	for rows.Next() {
		var a Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.SongCount, &a.CoverArt, &a.Starred, &a.StarredAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- write path

// UpsertTrack writes one scanned file into the library, creating artist/album
// rows on demand and persisting embedded cover art exactly once per album.
func (s *Store) UpsertTrack(t TrackInput) (string, error) {
	artistName := strings.TrimSpace(t.Artist)
	if artistName == "" {
		artistName = "未知艺术家"
	}
	albumArtist := strings.TrimSpace(t.AlbumArtist)
	if albumArtist == "" {
		albumArtist = artistName
	}
	albumName := strings.TrimSpace(t.Album)
	if albumName == "" {
		albumName = "未知专辑"
	}
	title := strings.TrimSpace(t.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(t.RelPath), filepath.Ext(t.RelPath))
	}

	aid := ArtistID(artistName)
	alid := AlbumID(albumArtist, albumName)
	sid := SongID(t.RelPath)
	ts := now()

	// A manual metadata edit should survive rescans. Keep the edited fields as
	// long as the underlying file itself has not changed; technical fields and
	// sidecar lyrics are still refreshed.
	var metaEdited, oldSize, oldMtime int64
	if err := s.db.QueryRow(`SELECT meta_edited, size, file_mtime FROM songs WHERE id=?`, sid).
		Scan(&metaEdited, &oldSize, &oldMtime); err == nil && metaEdited == 1 &&
		oldSize == t.Size && oldMtime == t.ModTime {
		_, err := s.db.Exec(`UPDATE songs SET path=?, duration=?, bitrate=?, sample_rate=?, channels=?,
				size=?, suffix=?, content_type=?,
				lyrics=CASE WHEN ?<>'' THEN ? ELSE lyrics END, file_mtime=?, updated_at=?
			WHERE id=?`,
			filepath.ToSlash(t.AbsPath), t.Duration, t.Bitrate, t.SampleRate, t.Channels,
			t.Size, t.Suffix, t.ContentType, t.Lyrics, t.Lyrics, t.ModTime, ts, sid)
		return sid, err
	}

	if _, err := s.db.Exec(`INSERT OR IGNORE INTO artists(id,name,created_at) VALUES(?,?,?)`, aid, artistName, ts); err != nil {
		return "", err
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO artists(id,name,created_at) VALUES(?,?,?)`, ArtistID(albumArtist), albumArtist, ts); err != nil {
		return "", err
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO albums(id,name,artist_id,artist,year,genre,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`, alid, albumName, ArtistID(albumArtist), albumArtist, t.Year, t.Genre, ts, ts); err != nil {
		return "", err
	}

	// Cover art: first embedded picture wins for the whole album.
	if len(t.CoverData) > 0 && s.dir != "" {
		var existing string
		_ = s.db.QueryRow(`SELECT cover_path FROM albums WHERE id=?`, alid).Scan(&existing)
		if existing == "" {
			ext := coverExt(t.CoverMime)
			rel := alid + ext
			full := filepath.Join(s.dir, rel)
			if err := os.WriteFile(full, t.CoverData, 0o644); err == nil {
				_, _ = s.db.Exec(`UPDATE albums SET cover_path=?, cover_mime=? WHERE id=?`, full, t.CoverMime, alid)
			}
		}
	}

	_, err := s.db.Exec(`INSERT INTO songs(id,path,title,artist_id,artist,album_id,album,album_artist,genre,
			track,disc,year,duration,bitrate,sample_rate,channels,size,suffix,content_type,lyrics,
			meta_edited,file_mtime,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path, title=excluded.title, artist_id=excluded.artist_id, artist=excluded.artist,
			album_id=excluded.album_id, album=excluded.album, album_artist=excluded.album_artist,
			genre=excluded.genre, track=excluded.track, disc=excluded.disc, year=excluded.year,
			duration=excluded.duration, bitrate=excluded.bitrate, sample_rate=excluded.sample_rate,
			channels=excluded.channels, size=excluded.size, suffix=excluded.suffix,
			content_type=excluded.content_type,
			lyrics=CASE WHEN excluded.lyrics<>'' THEN excluded.lyrics ELSE songs.lyrics END,
			meta_edited=0, file_mtime=excluded.file_mtime,
			updated_at=excluded.updated_at`,
		sid, filepath.ToSlash(t.AbsPath), title, aid, artistName, alid, albumName, albumArtist, t.Genre,
		t.Track, t.Disc, t.Year, t.Duration, t.Bitrate, t.SampleRate, t.Channels, t.Size, t.Suffix, t.ContentType,
		t.Lyrics, 0, t.ModTime, ts, ts)
	if err != nil {
		return "", err
	}
	return sid, nil
}

func coverExt(mime string) string {
	switch strings.ToLower(mime) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

// DeleteMissing removes songs whose files no longer exist on disk.
func (s *Store) DeleteMissing(seen map[string]bool) (int, error) {
	rows, err := s.db.Query(`SELECT id, path FROM songs`)
	if err != nil {
		return 0, err
	}
	var stale []string
	for rows.Next() {
		var id, p string
		if err := rows.Scan(&id, &p); err != nil {
			rows.Close()
			return 0, err
		}
		if !seen[id] {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		if err := s.DeleteSong(id); err != nil {
			return 0, err
		}
	}
	if len(stale) > 0 {
		if err := s.RefreshAlbumStats(); err != nil {
			return len(stale), err
		}
	}
	return len(stale), nil
}

func (s *Store) PathExists(path string) bool {
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM songs WHERE path=?`, filepath.ToSlash(path)).Scan(&n)
	return n > 0
}

// Exists reports whether a path is already in the library.
func (s *Store) Exists(path string) bool { return s.PathExists(path) }

func (s *Store) SongByPath(path string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM songs WHERE path=?`, filepath.ToSlash(path)).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// ArtistCover returns the cover of the artist's most recent album that has one.
func (s *Store) ArtistCover(artistID string) (string, string, error) {
	var path, mime string
	err := s.db.QueryRow(`SELECT cover_path, cover_mime FROM albums
		WHERE artist_id=? AND cover_path<>'' ORDER BY year DESC, created_at DESC LIMIT 1`, artistID).Scan(&path, &mime)
	return path, mime, err
}

// TouchNowPlaying records a "now playing" ping without incrementing play count.
func (s *Store) TouchNowPlaying(userID, songID string, at int64) error {
	_, err := s.db.Exec(`INSERT INTO annotations(user_id,item_id,item_type,play_count,play_date)
		VALUES(?,?,'song',0,?)
		ON CONFLICT(user_id,item_id) DO UPDATE SET play_date=excluded.play_date`, userID, songID, at)
	return err
}
