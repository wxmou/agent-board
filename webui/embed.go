package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the built Vue application. The Vite build writes directly to
// webui/dist, so the Go binary remains a single deployable artifact.
func FS() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
