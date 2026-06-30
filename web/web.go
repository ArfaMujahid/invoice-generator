// Package web embeds the server-rendered HTML templates and static assets
// (CSS, JS, images) so the application ships as a single self-contained binary
// (SRS §2.4, NFR-5). Nothing here contains business logic.
package web

import (
	"embed"
	"io/fs"
)

// templatesFS holds the raw template tree, kept unexported so callers go through
// Templates().
//
//go:embed templates/*.html
var templatesFS embed.FS

// staticFS holds the static asset tree (CSS and, later, JS/images).
//
//go:embed static
var staticFS embed.FS

// Templates returns the embedded HTML template tree rooted at "templates".
func Templates() fs.FS {
	return templatesFS
}

// Static returns the embedded static-asset tree rooted at "static", ready to be
// served with http.FileServerFS.
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// The path is a compile-time constant that exists, so this can only fail
		// if the embed directive is broken — a programmer error, not runtime.
		panic("web: static sub-filesystem: " + err.Error())
	}
	return sub
}
