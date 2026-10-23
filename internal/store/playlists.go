package store

import (
	"database/sql"
	"strings"
	"time"

	"mrome/internal/util"
)

func (s *Store) ListPlaylists(userID string) ([]Playlist, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.comment, p.owner_id, COALESCE(u.username,''), p.public, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM playlist_songs ps WHERE ps.playlist_id=p.id),
			(SELECT CAST(COALESCE(SUM(s.duration),0) AS INTEGER) FROM playlist_songs ps
				JOIN songs s ON s.id=ps.song_id WHERE ps.playlist_id=p.id),
			COALESCE((SELECT s.album_id FROM playlist_songs ps JOIN songs s ON s.id=ps.song_id
				WHERE ps.playlist_id=p.id ORDER BY ps.position LIMIT 1),'')
		FROM playlists p LEFT JOIN users u ON u.id=p.owner_id
		WHERE p.public=1 OR p.owner_id=?
		   OR EXISTS(SELECT 1 FROM users vu WHERE vu.id=? AND vu.is_admin=1)
		ORDER BY p.updated_at DESC`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Playlist{}
	for rows.Next() {
		var p Playlist
		var pub int
		if err := rows.Scan(&p.ID, &p.Name, &p.Comment, &p.OwnerID, &p.Owner, &pub, &p.Created, &p.Changed,
			&p.SongCount, &p.Duration, &p.CoverArt); err != nil {
			return nil, err
		}
		p.Public = pub == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPlaylist(userID, id string, withEntries bool) (*Playlist, error) {
	var p Playlist
	var pub int
	err := s.db.QueryRow(`
		SELECT p.id, p.name, p.comment, p.owner_id, COALESCE(u.username,''), p.public, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM playlist_songs ps WHERE ps.playlist_id=p.id),
			(SELECT CAST(COALESCE(SUM(s.duration),0) AS INTEGER) FROM playlist_songs ps
				JOIN songs s ON s.id=ps.song_id WHERE ps.playlist_id=p.id),
			COALESCE((SELECT s.album_id FROM playlist_songs ps JOIN songs s ON s.id=ps.song_id
				WHERE ps.playlist_id=p.id ORDER BY ps.position LIMIT 1),'')
		FROM playlists p LEFT JOIN users u ON u.id=p.owner_id
		WHERE p.id=? AND (p.public=1 OR p.owner_id=?
		   OR EXISTS(SELECT 1 FROM users vu WHERE vu.id=? AND vu.is_admin=1))`, id, userID, userID).
		Scan(&p.ID, &p.Name, &p.Comment, &p.OwnerID, &p.Owner, &pub, &p.Created, &p.Changed,
			&p.SongCount, &p.Duration, &p.CoverArt)
	if err != nil {
		return nil, err
	}
	p.Public = pub == 1
	if withEntries {
		rows, err := s.db.Query(`SELECT `+songCols+` FROM playlist_songs ps
			JOIN songs s ON s.id=ps.song_id
			LEFT JOIN annotations an ON an.item_id=s.id AND an.item_type='song' AND an.user_id=?
			WHERE ps.playlist_id=? ORDER BY ps.position`, userID, id)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		entries := []Song{}
		for rows.Next() {
			song, err := scanSong(rows)
			if err != nil {
				return nil, err
			}
			entries = append(entries, song)
		}
		p.Entries = entries
	}
	return &p, nil
}

func (s *Store) CreatePlaylist(userID, name, comment string, public bool, songIDs []string) (*Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "新建歌单"
	}
	id := util.MD5Hex("playlist", userID, name, time.Now().String())
	ts := time.Now().Unix()
	if _, err := s.db.Exec(`INSERT INTO playlists(id,name,comment,owner_id,public,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?)`, id, name, comment, userID, boolInt(public), ts, ts); err != nil {
		return nil, err
	}
	if len(songIDs) > 0 {
		if err := s.addSongs(id, songIDs); err != nil {
			return nil, err
		}
	}
	return s.GetPlaylist(userID, id, true)
}

func (s *Store) addSongs(playlistID string, songIDs []string) error {
	var pos int
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(position),0) FROM playlist_songs WHERE playlist_id=?`, playlistID).Scan(&pos)
	for _, sid := range songIDs {
		sid = strings.TrimSpace(sid)
		if sid == "" {
			continue
		}
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM songs WHERE id=?`, sid).Scan(&exists); err != nil || exists == 0 {
			continue
		}
		pos++
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO playlist_songs(playlist_id,song_id,position)
			VALUES(?,?,?)`, playlistID, sid, pos); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpdatePlaylistMeta(id, name string, comment *string, public *bool) error {
	if name != "" {
		if _, err := s.db.Exec(`UPDATE playlists SET name=? WHERE id=?`, name, id); err != nil {
			return err
		}
	}
	if comment != nil {
		if _, err := s.db.Exec(`UPDATE playlists SET comment=? WHERE id=?`, *comment, id); err != nil {
			return err
		}
	}
	if public != nil {
		if _, err := s.db.Exec(`UPDATE playlists SET public=? WHERE id=?`, boolInt(*public), id); err != nil {
			return err
		}
	}
	return s.touchPlaylist(id)
}

func (s *Store) touchPlaylist(id string) error {
	_, err := s.db.Exec(`UPDATE playlists SET updated_at=? WHERE id=?`, time.Now().Unix(), id)
	return err
}

func (s *Store) AddPlaylistSongs(id string, songIDs []string) error {
	if err := s.addSongs(id, songIDs); err != nil {
		return err
	}
	return s.touchPlaylist(id)
}

// RemovePlaylistIndexes removes entries by their zero-based position in the list.
func (s *Store) RemovePlaylistIndexes(id string, indexes []int) error {
	rows, err := s.db.Query(`SELECT song_id FROM playlist_songs WHERE playlist_id=? ORDER BY position`, id)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, sid)
	}
	rows.Close()
	drop := map[string]bool{}
	for _, idx := range indexes {
		if idx >= 0 && idx < len(ids) {
			drop[ids[idx]] = true
		}
	}
	for sid := range drop {
		if _, err := s.db.Exec(`DELETE FROM playlist_songs WHERE playlist_id=? AND song_id=?`, id, sid); err != nil {
			return err
		}
	}
	return s.touchPlaylist(id)
}

func (s *Store) DeletePlaylist(id string) error {
	if _, err := s.db.Exec(`DELETE FROM playlist_songs WHERE playlist_id=?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM playlists WHERE id=?`, id)
	return err
}

func (s *Store) PlaylistOwner(id string) (string, error) {
	var owner string
	err := s.db.QueryRow(`SELECT owner_id FROM playlists WHERE id=?`, id).Scan(&owner)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return owner, err
}

// RemovePlaylistSong removes a single entry from a playlist.
func (s *Store) RemovePlaylistSong(playlistID, songID string) error {
	if _, err := s.db.Exec(`DELETE FROM playlist_songs WHERE playlist_id=? AND song_id=?`, playlistID, songID); err != nil {
		return err
	}
	return s.touchPlaylist(playlistID)
}
