//go:build embed_frontend

package main

import (
	"embed"
	"io/fs"

	"github.com/owen282000/life-dashboard-hub/internal/server"
)

//go:embed all:dist
var frontendFS embed.FS

func initEmbedFrontend() {
	sub, err := fs.Sub(frontendFS, "dist")
	if err == nil {
		server.EmbedFrontendFS = sub
	}
}
