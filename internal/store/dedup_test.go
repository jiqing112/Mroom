package store

import (
	"os"
	"path/filepath"
	"testing"
)

func makeTestFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func trackIn(abs, rel string) TrackInput {
	return TrackInput{
		RelPath: rel, AbsPath: abs, Size: int64(len(rel) + 100), ModTime: 100,
		Title: "T", Artist: "A", Album: "AL", AlbumArtist: "AA", MD5: "hash-" + rel,
	}
}

// UpsertTrack must persist the content hash, and a rescan of an unchanged
// file must keep the old hash instead of dropping it.
func TestUpsertTrackKeepsMD5(t *testing.T) {
	s, _ := openTestStore(t)
	in := trackIn("C:/music/a.mp3", "a.mp3")
	if _, err := s.UpsertTrack(in); err != nil {
		t.Fatal(err)
	}
	md5, size, mtime, ok := s.StoredFileInfo("C:/music/a.mp3")
	if !ok || md5 != "hash-a.mp3" || size != in.Size || mtime != in.ModTime {
		t.Fatalf("md5 not stored: %q %v %v", md5, size, mtime)
	}
	// Rescan without a hash (scanner skips re-hash for unchanged files).
	in.MD5 = ""
	if _, err := s.UpsertTrack(in); err != nil {
		t.Fatal(err)
	}
	if md5, _, _, _ := s.StoredFileInfo("C:/music/a.mp3"); md5 != "hash-a.mp3" {
		t.Fatalf("rescan lost md5: %q", md5)
	}
}

// Identical content must be detectable, no matter which path it lives under.
func TestSongIDByMD5AndDuplicates(t *testing.T) {
	s, _ := openTestStore(t)
	mk := func(rel, hash string) {
		in := trackIn("C:/music/"+rel, rel)
		in.MD5 = hash
		if _, err := s.UpsertTrack(in); err != nil {
			t.Fatal(err)
		}
	}
	mk("one.mp3", "hashX")
	mk("two.mp3", "hashY")
	mk("copy.mp3", "hashX")
	mk("copy3.mp3", "hashX")

	if id, found := s.SongIDByMD5("hashY"); !found || id == "" {
		t.Fatal("existing hash not found")
	}
	if _, found := s.SongIDByMD5("hashZ"); found {
		t.Fatal("unknown hash must not match")
	}
	if _, found := s.SongIDByMD5(""); found {
		t.Fatal("empty hash must not match anything")
	}

	groups, err := s.MD5Duplicates()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].MD5 != "hashX" {
		t.Fatalf("expected one hashX group, got %+v", groups)
	}
	if len(groups[0].Extra) != 2 {
		t.Fatalf("expected 2 extras, got %+v", groups[0].Extra)
	}
	for _, id := range groups[0].Extra {
		if err := s.DeleteSong(id); err != nil {
			t.Fatal(err)
		}
	}
	groups, _ = s.MD5Duplicates()
	if len(groups) != 0 {
		t.Fatalf("duplicates not cleared: %+v", groups)
	}
}
