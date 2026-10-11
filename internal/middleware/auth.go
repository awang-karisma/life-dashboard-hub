package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/owen282000/life-dashboard-hub/internal/auth"
)

func RequireAuth(sessionService *auth.SessionService) fiber.Handler {
	return func(c fiber.Ctx) error {
		rawToken := c.Cookies("session")
		if rawToken == "" {
			// Check Authorization header
			authHeader := string(c.Request().Header.Peek("Authorization"))
			if strings.HasPrefix(authHeader, "Bearer ") {
				rawToken = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if rawToken == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "authentication required",
			})
		}

		user, _, err := sessionService.ValidateSession(rawToken)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid or expired session",
			})
		}

		c.Locals("userID", user.ID)
		c.Locals("user", user)
		return c.Next()
	}
}
