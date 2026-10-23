package api

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"mrome/internal/config"
)

func TestCoverThumbnailResizeAndCache(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cover.png")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 120, 255})
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	s := &Server{Cfg: config.Config{CoversDir: dir}}
	out, mime, err := s.coverThumbnail(src, 160)
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" {
		t.Fatalf("mime = %q, want image/png", mime)
	}
	tf, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer tf.Close()
	thumb, _, err := image.Decode(tf)
	if err != nil {
		t.Fatal(err)
	}
	b := thumb.Bounds()
	if b.Dx() > 160 || b.Dy() > 160 {
		t.Fatalf("thumbnail is %dx%d, want <=160", b.Dx(), b.Dy())
	}
	if b.Dx() != 160 || b.Dy() != 120 {
		t.Fatalf("aspect ratio not preserved: %dx%d", b.Dx(), b.Dy())
	}
	// Second call should hit the cache and return the same path.
	out2, _, err := s.coverThumbnail(src, 160)
	if err != nil {
		t.Fatal(err)
	}
	if out2 != out {
		t.Fatalf("cache miss: %q != %q", out2, out)
	}
}
