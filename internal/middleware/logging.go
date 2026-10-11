package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

func Logger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		ip := c.IP()

		if err != nil {
			slog.Error("HTTP request failed",
				"status", status,
				"method", method,
				"path", path,
				"ip", ip,
				"duration", duration,
				"error", err,
			)
			return err
		}

		if status >= 400 {
			slog.Warn("HTTP request",
				"status", status,
				"method", method,
				"path", path,
				"ip", ip,
				"duration", duration,
			)
		} else {
			slog.Info("HTTP request",
				"status", status,
				"method", method,
				"path", path,
				"ip", ip,
				"duration", duration,
			)
		}

		return nil
	}
}
