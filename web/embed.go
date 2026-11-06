// Package web embeds the built frontend so a single binary can serve it.
// The dist directory normally only exists after `npm run build`; a committed
// .gitkeep keeps the embed pattern compilable on a fresh checkout.
package web

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:dist
var dist embed.FS

// FS returns the frontend asset tree. An overrideDir that holds a built
// frontend (i.e. contains index.html) wins, so a development build on disk
// can shadow the embedded one and be rebuilt without restarting the process;
// otherwise the assets compiled into this binary are served.
func FS(overrideDir string) (fs.FS, bool) {
	if overrideDir != "" {
		if _, err := os.Stat(filepath.Join(overrideDir, "index.html")); err == nil {
			return os.DirFS(overrideDir), false
		}
	}
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Unreachable: "dist" is a directory baked in at compile time.
		panic(err)
	}
	return sub, true
}
