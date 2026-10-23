package store

import (
	"os"
	"path/filepath"
	"testing"
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
	if err := s.SetRating("user", id, 5); err != nil {
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
