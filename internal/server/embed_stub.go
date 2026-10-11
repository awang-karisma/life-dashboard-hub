//go:build !embed_frontend

package server

import "github.com/gofiber/fiber/v3"

func MountFrontend(app *fiber.App) {
	// In separate/proxy mode, frontend is served by external proxy (Caddy/Nginx) or Vite dev server
}
