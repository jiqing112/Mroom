package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	dir    string
	secret []byte
}

func Open(path string) (*Store, error) {
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(8000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	secret := s.GetSetting("secret", "")
	if secret == "" {
		secret = randomHex(32)
		if err := s.SetSetting("secret", secret); err != nil {
			return nil, err
		}
	}
	s.secret = []byte(secret)
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id            TEXT PRIMARY KEY,
  username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  password_enc  TEXT NOT NULL DEFAULT '',
  is_admin      INTEGER NOT NULL DEFAULT 0,
  api_key       TEXT NOT NULL DEFAULT '',
  session_version INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS artists (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  sort_name  TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_artists_name ON artists(name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS albums (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  artist_id  TEXT NOT NULL DEFAULT '',
  artist     TEXT NOT NULL DEFAULT '',
  year       INTEGER NOT NULL DEFAULT 0,
  genre      TEXT NOT NULL DEFAULT '',
  cover_path TEXT NOT NULL DEFAULT '',
  cover_mime TEXT NOT NULL DEFAULT '',
  song_count INTEGER NOT NULL DEFAULT 0,
  duration   INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_albums_artist ON albums(artist_id);
CREATE INDEX IF NOT EXISTS idx_albums_created ON albums(created_at);

CREATE TABLE IF NOT EXISTS songs (
  id           TEXT PRIMARY KEY,
  path         TEXT NOT NULL UNIQUE,
  title        TEXT NOT NULL,
  artist_id    TEXT NOT NULL DEFAULT '',
  artist       TEXT NOT NULL DEFAULT '',
  album_id     TEXT NOT NULL DEFAULT '',
  album        TEXT NOT NULL DEFAULT '',
  album_artist TEXT NOT NULL DEFAULT '',
  genre        TEXT NOT NULL DEFAULT '',
  track        INTEGER NOT NULL DEFAULT 0,
  disc         INTEGER NOT NULL DEFAULT 0,
  year         INTEGER NOT NULL DEFAULT 0,
  duration     REAL NOT NULL DEFAULT 0,
  bitrate      INTEGER NOT NULL DEFAULT 0,
  sample_rate  INTEGER NOT NULL DEFAULT 0,
  channels     INTEGER NOT NULL DEFAULT 0,
  size         INTEGER NOT NULL DEFAULT 0,
  suffix       TEXT NOT NULL DEFAULT '',
  content_type TEXT NOT NULL DEFAULT '',
  lyrics       TEXT NOT NULL DEFAULT '',
  meta_edited  INTEGER NOT NULL DEFAULT 0,
  file_mtime   INTEGER NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_songs_album ON songs(album_id);
CREATE INDEX IF NOT EXISTS idx_songs_artist ON songs(artist_id);
CREATE INDEX IF NOT EXISTS idx_songs_created ON songs(created_at);
CREATE INDEX IF NOT EXISTS idx_songs_title ON songs(title COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS playlists (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  comment    TEXT NOT NULL DEFAULT '',
  owner_id   TEXT NOT NULL DEFAULT '',
  public     INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS playlist_songs (
  playlist_id TEXT NOT NULL,
  song_id     TEXT NOT NULL,
  position    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (playlist_id, song_id)
);
CREATE INDEX IF NOT EXISTS idx_pls_song ON playlist_songs(song_id);

CREATE TABLE IF NOT EXISTS annotations (
  user_id    TEXT NOT NULL,
  item_id    TEXT NOT NULL,
  item_type  TEXT NOT NULL,
  starred    INTEGER NOT NULL DEFAULT 0,
  starred_at INTEGER NOT NULL DEFAULT 0,
  rating     INTEGER NOT NULL DEFAULT 0,
  play_count INTEGER NOT NULL DEFAULT 0,
  play_date  INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_ann_type ON annotations(user_id, item_type, starred);

CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);
`

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	// Columns added after the initial schema need explicit ALTERs for
	// databases created by older builds.
	for _, col := range []struct{ table, name, def string }{
		{"songs", "meta_edited", "INTEGER NOT NULL DEFAULT 0"},
		{"songs", "file_mtime", "INTEGER NOT NULL DEFAULT 0"},
		{"users", "session_version", "INTEGER NOT NULL DEFAULT 0"},
	} {
		ok, err := s.hasColumn(col.table, col.name)
		if err != nil {
			return err
		}
		if !ok {
			if _, err := s.db.Exec(`ALTER TABLE ` + col.table + ` ADD COLUMN ` + col.name + ` ` + col.def); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) hasColumn(table, column string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) GetSetting(key, def string) string {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) CountSongs() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM songs`).Scan(&n)
	return n, err
}

// PruneEmpty removes albums/artists left without songs and any related data:
// playlist entries, annotations and the now unused cover files.
func (s *Store) PruneEmpty() error {
	rows, err := s.db.Query(`SELECT cover_path FROM albums
		WHERE cover_path <> '' AND id NOT IN (SELECT DISTINCT album_id FROM songs)`)
	if err != nil {
		return err
	}
	var covers []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		covers = append(covers, p)
	}
	rows.Close()

	if _, err := s.db.Exec(`DELETE FROM albums WHERE id NOT IN (SELECT DISTINCT album_id FROM songs)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM artists WHERE id NOT IN (SELECT DISTINCT artist_id FROM songs)
		AND id NOT IN (SELECT DISTINCT artist_id FROM albums)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM playlist_songs WHERE song_id NOT IN (SELECT id FROM songs)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM annotations
		WHERE (item_type='song'   AND item_id NOT IN (SELECT id FROM songs))
		   OR (item_type='album'  AND item_id NOT IN (SELECT id FROM albums))
		   OR (item_type='artist' AND item_id NOT IN (SELECT id FROM artists))`); err != nil {
		return err
	}
	if s.dir != "" {
		for _, p := range covers {
			rel, err := filepath.Rel(s.dir, p)
			if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
				continue
			}
			_ = os.Remove(p)
			// Drop cached thumbnails for the same cover as well.
			base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
			if thumbs, err := filepath.Glob(filepath.Join(s.dir, "thumbs", base+"-*")); err == nil {
				for _, thumb := range thumbs {
					_ = os.Remove(thumb)
				}
			}
		}
	}
	return nil
}

func (s *Store) DeleteSong(id string) error {
	if _, err := s.db.Exec(`DELETE FROM songs WHERE id=?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM playlist_songs WHERE song_id=?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM annotations WHERE item_type='song' AND item_id=?`, id)
	return err
}

func (s *Store) Stats() (map[string]int, error) {
	out := map[string]int{}
	rows, err := s.db.Query(`SELECT
		(SELECT COUNT(*) FROM songs),
		(SELECT COUNT(*) FROM albums),
		(SELECT COUNT(*) FROM artists),
		(SELECT COUNT(*) FROM playlists),
		(SELECT COALESCE(SUM(duration),0) FROM songs)`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var total float64
	var songs, albums, artists, playlists int
	if rows.Next() {
		if err := rows.Scan(&songs, &albums, &artists, &playlists, &total); err != nil {
			return out, err
		}
	}
	out["songs"] = songs
	out["albums"] = albums
	out["artists"] = artists
	out["playlists"] = playlists
	out["duration"] = int(total)
	return out, nil
}

func like(q string) string {
	q = strings.ReplaceAll(q, "%", "")
	q = strings.ReplaceAll(q, "_", "")
	return "%" + q + "%"
}

func now() int64 { return time.Now().Unix() }

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

var (
	errNoRows   = sql.ErrNoRows
	ErrNotFound = fmt.Errorf("not found")
)

func wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}
