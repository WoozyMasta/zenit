// Package assets provides access to embedded static files such as SQL, CSS, JS, images, and HTML templates.
package assets

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed css/*.min.css data/*.min.json data/site.webmanifest img/*.png img/*.svg js/*.min.js migrations/*.sql *.min.html favicon.ico
var embedFS embed.FS

// GetFileSystem returns an http.FileSystem interface for the embedded assets,
// rooted at the current directory of the embed.FS.
func GetFileSystem() http.FileSystem {
	fsys, err := fs.Sub(embedFS, ".")
	if err != nil {
		panic(err)
	}
	return http.FS(fsys)
}

// ReadFile returns the content of a specific file from the embedded assets by its name.
func ReadFile(name string) ([]byte, error) {
	return embedFS.ReadFile(name)
}

// ReadDir returns the directory entries for a specific path.
func ReadDir(name string) ([]fs.DirEntry, error) {
	return embedFS.ReadDir(name)
}
