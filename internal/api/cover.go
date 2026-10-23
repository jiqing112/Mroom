package api

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var coverThumbMu sync.Mutex

// coverThumbnail decodes the original artwork, resizes it to fit a square of
// the requested size and caches the result next to the covers. Cached files are
// reused until the source image changes.
func (s *Server) coverThumbnail(src string, size int) (string, string, error) {
	if size <= 0 || s.Cfg.CoversDir == "" {
		return "", "", errors.New("invalid thumbnail request")
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(s.Cfg.CoversDir, "thumbs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))

	// The extension is decided after decoding (PNG keeps transparency), so
	// check both possible cache entries before taking the lock.
	for _, ext := range []string{".jpg", ".png"} {
		out := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, size, ext))
		if st, err := os.Stat(out); err == nil && !st.ModTime().Before(srcInfo.ModTime()) {
			return out, mimeForExt(ext), nil
		}
	}

	coverThumbMu.Lock()
	defer coverThumbMu.Unlock()
	// Another request may have generated it while we waited.
	for _, ext := range []string{".jpg", ".png"} {
		out := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, size, ext))
		if st, err := os.Stat(out); err == nil && !st.ModTime().Before(srcInfo.ModTime()) {
			return out, mimeForExt(ext), nil
		}
	}

	f, err := os.Open(src)
	if err != nil {
		return "", "", err
	}
	img, format, err := image.Decode(f)
	f.Close()
	if err != nil {
		return "", "", err
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return "", "", errors.New("empty cover image")
	}
	outSize := size
	if maxDim := max(b.Dx(), b.Dy()); outSize > maxDim {
		outSize = maxDim
	}
	w, h := outSize, outSize
	if b.Dx() >= b.Dy() {
		h = max(1, int(float64(outSize)*float64(b.Dy())/float64(b.Dx())))
	} else {
		w = max(1, int(float64(outSize)*float64(b.Dx())/float64(b.Dy())))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)

	ext := ".jpg"
	if format == "png" || format == "gif" {
		ext = ".png"
	}
	out := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, size, ext))
	tmp, err := os.CreateTemp(dir, ".thumb-*"+ext)
	if err != nil {
		return "", "", err
	}
	tmpName := tmp.Name()
	if ext == ".png" {
		err = png.Encode(tmp, dst)
	} else {
		err = jpeg.Encode(tmp, dst, &jpeg.Options{Quality: 86})
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmpName)
		return "", "", err
	}
	if err := os.Rename(tmpName, out); err != nil {
		_ = os.Remove(tmpName)
		return "", "", err
	}
	return out, mimeForExt(ext), nil
}

func mimeForExt(ext string) string {
	if ext == ".png" {
		return "image/png"
	}
	return "image/jpeg"
}
