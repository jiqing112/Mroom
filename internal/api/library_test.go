package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupAlbumFolder(t *testing.T) {
	root := t.TempDir()
	album := filepath.Join(root, "Artist", "Album")
	trash := filepath.Join(root, "trash")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(trash, 0o755); err != nil {
		t.Fatal(err)
	}
	cover := filepath.Join(album, "cover.jpg")
	song := filepath.Join(album, "track.mp3")
	if err := os.WriteFile(cover, []byte("cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(song, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Another audio file remains: the artwork must stay.
	cleanupAlbumFolder(song, trash)
	if _, err := os.Stat(cover); err != nil {
		t.Fatalf("cover moved while audio files remain: %v", err)
	}

	// Last audio file gone: artwork moves to trash and the folder disappears.
	if err := os.Remove(song); err != nil {
		t.Fatal(err)
	}
	cleanupAlbumFolder(song, trash)
	if _, err := os.Stat(cover); !os.IsNotExist(err) {
		t.Fatalf("cover was not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(trash, "cover.jpg")); err != nil {
		t.Fatalf("cover missing from trash: %v", err)
	}
	if _, err := os.Stat(album); !os.IsNotExist(err) {
		t.Fatalf("empty album folder was not removed: %v", err)
	}
}
