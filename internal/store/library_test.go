package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	covers := filepath.Join(dir, "covers")
	if err := os.MkdirAll(covers, 0o755); err != nil {
		t.Fatal(err)
	}
	s.SetCoversDir(covers)
	return s, dir
}

func makeTrackInput(t *testing.T, dir string) TrackInput {
	t.Helper()
	path := filepath.Join(dir, "Artist", "Album", "song.mp3")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, 128), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return TrackInput{
		RelPath:     "Artist/Album/song.mp3",
		AbsPath:     path,
		Size:        st.Size(),
		ModTime:     st.ModTime().Unix(),
		Title:       "Tagged Title",
		Artist:      "Track Artist",
		AlbumArtist: "Album Artist",
		Album:       "Album",
		Genre:       "Rock",
		Track:       1,
		Year:        2020,
		Duration:    12.5,
		Bitrate:     192,
		Suffix:      "mp3",
		ContentType: "audio/mpeg",
	}
}

func TestMetadataSurvivesRescanUntilFileChanges(t *testing.T) {
	s, dir := openTestStore(t)
	in := makeTrackInput(t, dir)
	id, err := s.UpsertTrack(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSongMeta(id, "Edited Title", "Track Artist", "", "Album", "Rock", 2021, 2); err != nil {
		t.Fatal(err)
	}
	song, err := s.GetSong("", id)
	if err != nil {
		t.Fatal(err)
	}
	if song.Title != "Edited Title" || song.AlbumArtist != "Album Artist" {
		t.Fatalf("edit not applied correctly: %+v", song)
	}

	// Same file: a rescan must keep the manual edit.
	if _, err := s.UpsertTrack(in); err != nil {
		t.Fatal(err)
	}
	song, _ = s.GetSong("", id)
	if song.Title != "Edited Title" {
		t.Fatalf("metadata edit lost on rescan: %q", song.Title)
	}

	// Changed file: tags win again.
	in.ModTime += 10
	if _, err := s.UpsertTrack(in); err != nil {
		t.Fatal(err)
	}
	song, _ = s.GetSong("", id)
	if song.Title != "Tagged Title" {
		t.Fatalf("tag metadata did not replace edit after file change: %q", song.Title)
	}
}

func TestMovingTrackCarriesAndPrunesCover(t *testing.T) {
	s, dir := openTestStore(t)
	in := makeTrackInput(t, dir)
	in.CoverData = []byte("fake-jpeg")
	in.CoverMime = "image/jpeg"
	id, err := s.UpsertTrack(in)
	if err != nil {
		t.Fatal(err)
	}
	old, err := s.GetSong("", id)
	if err != nil {
		t.Fatal(err)
	}
	oldCover, _, err := s.AlbumCoverPath(old.AlbumID)
	if err != nil || oldCover == "" {
		t.Fatalf("old album cover missing: %v %q", err, oldCover)
	}

	if err := s.UpdateSongMeta(id, "Moved", "Track Artist", "Album Artist", "New Album", "Rock", 2020, 1); err != nil {
		t.Fatal(err)
	}
	song, _ := s.GetSong("", id)
	if song.AlbumID == old.AlbumID {
		t.Fatal("album did not change")
	}
	newCover, _, err := s.AlbumCoverPath(song.AlbumID)
	if err != nil || newCover == "" {
		t.Fatalf("cover was not copied to new album: %v %q", err, newCover)
	}
	if _, err := os.Stat(newCover); err != nil {
		t.Fatalf("copied cover file missing: %v", err)
	}
	// The now empty source album and its cover should have been pruned.
	if _, err := os.Stat(oldCover); !os.IsNotExist(err) {
		t.Fatalf("old cover was not removed: %v", err)
	}
}

func TestDeletePrunesAnnotationsPlaylistAndCover(t *testing.T) {
	s, dir := openTestStore(t)
	in := makeTrackInput(t, dir)
	in.CoverData = []byte("fake-jpeg")
	in.CoverMime = "image/jpeg"
	id, err := s.UpsertTrack(in)
	if err != nil {
		t.Fatal(err)
	}
	cover, _, err := s.AlbumCoverPath(AlbumID("Album Artist", "Album"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetStar("user", id, "song", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRating("user", id, "song", 5); err != nil {
		t.Fatal(err)
	}
	if err := s.Scrobble("user", id, 123); err != nil {
		t.Fatal(err)
	}
	pl, err := s.CreatePlaylist("user", "Test", "", true, []string{id})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSong(id); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshAlbumStats(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM annotations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("annotations left after delete: %d", n)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM playlist_songs WHERE playlist_id=?`, pl.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("playlist entries left after delete: %d", n)
	}
	if _, err := os.Stat(cover); !os.IsNotExist(err) {
		t.Fatalf("cover file not removed: %v", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM albums`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("empty album left behind: %d", n)
	}
}

func TestSongIDPreservesCaseAndRehashMigrates(t *testing.T) {
	s, dir := openTestStore(t)
	in := makeTrackInput(t, dir) // rel path "Artist/Album/song.mp3"
	in.CoverData = []byte("fake-jpeg")
	in.CoverMime = "image/jpeg"
	id, err := s.UpsertTrack(in)
	if err != nil {
		t.Fatal(err)
	}
	// Two paths that differ only in case must derive distinct ids, or one of
	// the two files becomes unreachable on case-sensitive filesystems.
	sibling := in
	sibling.RelPath = "artist/album/Song.mp3"
	sibling.AbsPath = filepath.Join(dir, "artist", "album", "Song.mp3")
	sibling.Title = "Case Sibling"
	if SongID(in.RelPath) == SongID(sibling.RelPath) {
		t.Fatal("case-differing paths collapsed to one song id")
	}
	sid2, err := s.UpsertTrack(sibling)
	if err != nil {
		t.Fatal(err)
	}
	if sid2 == id {
		t.Fatal("case-differing files shared one row")
	}

	// Star the first track, then simulate an old database by forcing a
	// lowercase id onto it; the rehash must move annotations along.
	if err := s.SetStar("user", id, "song", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePlaylist("user", "P", "", true, []string{id}); err != nil {
		t.Fatal(err)
	}
	lower := SongID(strings.ToLower(in.RelPath))
	if lower == id {
		t.Fatal("expected the legacy lowercase id to differ")
	}
	for _, q := range [][2]any{
		{`UPDATE songs SET id=? WHERE id=?`, lower},
		{`UPDATE playlist_songs SET song_id=? WHERE song_id=?`, lower},
		{`UPDATE annotations SET item_id=? WHERE item_id=? AND item_type='song'`, lower},
	} {
		if _, err := s.db.Exec(q[0].(string), q[1], id); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.RehashSongIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rehash moved %d rows, want 1", n)
	}
	song, err := s.GetSong("", id)
	if err != nil {
		t.Fatalf("rehashed row not reachable under its new id: %v", err)
	}
	if song.Title != "Tagged Title" {
		t.Fatalf("unexpected row after rehash: %+v", song)
	}
	var starred int
	if err := s.db.QueryRow(`SELECT COALESCE(starred,0) FROM annotations WHERE user_id='user' AND item_id=? AND item_type='song'`, id).Scan(&starred); err != nil {
		t.Fatalf("star annotation did not follow the rehash: %v", err)
	}
	if starred != 1 {
		t.Fatal("star annotation lost in rehash")
	}
}

func TestPasswordChangeBumpsSessionVersion(t *testing.T) {
	s, _ := openTestStore(t)
	u, err := s.CreateUser("tester", "first-pass", false)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.GetUserByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateUser(u.ID, "second-pass", nil); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetUserByID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.SessionVersion != before.SessionVersion+1 {
		t.Fatalf("session version = %d, want %d", after.SessionVersion, before.SessionVersion+1)
	}
	if _, err := s.Authenticate("tester", "second-pass"); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
	if _, err := s.Authenticate("tester", "first-pass"); err == nil {
		t.Fatal("old password still accepted")
	}
}

func TestCarryOverMovesPreservesIdentity(t *testing.T) {
	s, dir := openTestStore(t)
	in := makeTrackInput(t, dir)
	in.MD5 = "aaaa1111aaaa1111aaaa1111aaaa1111"
	id, err := s.UpsertTrack(in)
	if err != nil {
		t.Fatal(err)
	}
	var createdBefore int64
	if err := s.db.QueryRow(`SELECT created_at FROM songs WHERE id=?`, id).Scan(&createdBefore); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStar("user", id, "song", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRating("user", id, "song", 3); err != nil {
		t.Fatal(err)
	}
	if err := s.Scrobble("user", id, 777); err != nil {
		t.Fatal(err)
	}
	pl, err := s.CreatePlaylist("user", "P", "", true, []string{id})
	if err != nil {
		t.Fatal(err)
	}

	// The file moves to another folder: this scan ingests it as a fresh row.
	moved := in
	moved.RelPath = "NewFolder/Album/song.mp3"
	moved.AbsPath = filepath.Join(dir, "NewFolder", "Album", "song.mp3")
	freshID, err := s.UpsertTrack(moved)
	if err != nil {
		t.Fatal(err)
	}
	if freshID == id {
		t.Fatal("moved file should start as a fresh row")
	}

	scanStart := time.Now().Add(-time.Minute).Unix()
	n, err := s.CarryOverMoves(map[string]bool{filepath.ToSlash(moved.AbsPath): true}, scanStart)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("carryover reconciled %d rows, want 1", n)
	}

	// The fresh row is gone; the original id now lives at the new path.
	if _, err := s.GetSong("", freshID); err == nil {
		t.Fatal("fresh row should have been merged away")
	}
	rowF, err := s.GetSongFile(id)
	if err != nil {
		t.Fatalf("original id vanished: %v", err)
	}
	if rowF.Path != filepath.ToSlash(moved.AbsPath) {
		t.Fatalf("path = %q, want the moved path", rowF.Path)
	}
	var createdAfter int64
	if err := s.db.QueryRow(`SELECT created_at FROM songs WHERE id=?`, id).Scan(&createdAfter); err != nil {
		t.Fatal(err)
	}
	if createdAfter != createdBefore {
		t.Fatalf("created_at changed on move: %d -> %d", createdBefore, createdAfter)
	}
	var starred, rating, plays int
	if err := s.db.QueryRow(`SELECT COALESCE(starred,0), COALESCE(rating,0), COALESCE(play_count,0)
		FROM annotations WHERE user_id='user' AND item_id=? AND item_type='song'`, id).
		Scan(&starred, &rating, &plays); err != nil {
		t.Fatalf("annotations did not survive the move: %v", err)
	}
	if starred != 1 || rating != 3 || plays != 1 {
		t.Fatalf("annotations wrong after move: star=%d rating=%d plays=%d", starred, rating, plays)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM playlist_songs WHERE playlist_id=? AND song_id=?`, pl.ID, id).
		Scan(&plays); err != nil || plays != 1 {
		t.Fatalf("playlist membership lost on move: %v n=%d", err, plays)
	}

	// Rescanning the new path must update the surviving row in place instead of
	// spawning a duplicate keyed by the new path.
	again, err := s.UpsertTrack(moved)
	if err != nil {
		t.Fatal(err)
	}
	if again != id {
		t.Fatalf("rescan of moved path produced id %q, want %q", again, id)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM songs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("songs count = %d, want 1", count)
	}
}

func TestCarryOverMovesSkipsAmbiguousAndHashless(t *testing.T) {
	s, dir := openTestStore(t)
	// One stale row and two fresh rows sharing one hash: ambiguous, skip.
	in := makeTrackInput(t, dir)
	in.MD5 = "bbbb2222bbbb2222bbbb2222bbbb2222"
	staleID, err := s.UpsertTrack(in)
	if err != nil {
		t.Fatal(err)
	}
	var freshIDs []string
	for i, rel := range []string{"Dup/one.mp3", "Dup/two.mp3"} {
		dup := in
		dup.RelPath = rel
		dup.AbsPath = filepath.Join(dir, filepath.FromSlash(rel))
		fid, err := s.UpsertTrack(dup)
		if err != nil {
			t.Fatal(err)
		}
		freshIDs = append(freshIDs, fid)
		_ = i
	}
	seen := map[string]bool{}
	for _, rel := range []string{"Dup/one.mp3", "Dup/two.mp3"} {
		seen[filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(rel)))] = true
	}
	n, err := s.CarryOverMoves(seen, time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("ambiguous move reconciled %d rows, want 0", n)
	}
	if _, err := s.GetSong("", staleID); err != nil {
		t.Fatal("stale row should be untouched")
	}

	// A stale row without a hash has nothing to match on and stays put.
	if _, err := s.db.Exec(`UPDATE songs SET md5='' WHERE id=?`, staleID); err != nil {
		t.Fatal(err)
	}
	n, err = s.CarryOverMoves(seen, time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("hashless stale row reconciled %d rows, want 0", n)
	}
}
