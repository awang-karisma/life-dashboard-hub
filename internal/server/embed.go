//go:build embed_frontend

package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

var EmbedFrontendFS fs.FS

func MountFrontend(app *fiber.App) {
	if EmbedFrontendFS == nil {
		return
	}
	distSubFS := EmbedFrontendFS
	app.Use("/", static.New("", static.Config{
		FS:         distSubFS,
		IndexNames: []string{"index.html"},
	}))

	// Fallback to index.html for SPA routes that don't match API or static files
	app.Use(func(c fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/webhooks") || strings.HasPrefix(path, "/swagger") {
			return c.Next()
		}

		indexData, err := fs.ReadFile(distSubFS, "index.html")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Frontend build not found")
		}

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(http.StatusOK).Send(indexData)
	})
}
