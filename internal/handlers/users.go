package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/owen282000/life-dashboard-hub/internal/auth"
	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type UsersHandler struct {
	DB             *database.DB
	SessionService *auth.SessionService
}

func NewUsersHandler(db *database.DB, sessionService *auth.SessionService) *UsersHandler {
	return &UsersHandler{DB: db, SessionService: sessionService}
}

type UpdateUserRequest struct {
	DisplayName     *string `json:"display_name,omitempty"`
	Timezone        *string `json:"timezone,omitempty"`
	CurrentPassword *string `json:"current_password,omitempty"`
	NewPassword     *string `json:"new_password,omitempty"`
}

// GetCurrentUser godoc
// @Summary      Get current authenticated user profile
// @Description  Returns user profile including auth_method
// @Tags         users
// @Produce      json
// @Success      200 {object} UserResponse
// @Failure      401 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Router       /users/me [get]
func (h *UsersHandler) GetCurrentUser(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "user not found"})
	}

	return c.JSON(toUserResponse(&user))
}

// UpdateCurrentUser godoc
// @Summary      Update user profile
// @Description  Update display name, timezone, or change password
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body body UpdateUserRequest true "Update payload"
// @Success      200 {object} UserResponse
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse
// @Router       /users/me [patch]
func (h *UsersHandler) UpdateCurrentUser(c fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "unauthorized"})
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "user not found"})
	}

	var req UpdateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	if req.DisplayName != nil {
		trimmed := strings.TrimSpace(*req.DisplayName)
		if trimmed != "" {
			user.DisplayName = trimmed
		}
	}

	if req.Timezone != nil {
		user.Timezone = strings.TrimSpace(*req.Timezone)
	}

	// Handle password change
	if req.NewPassword != nil && *req.NewPassword != "" {
		if user.AuthMethod != "local" {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "password cannot be changed on an SSO/OIDC account",
			})
		}

		if req.CurrentPassword == nil || *req.CurrentPassword == "" {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "current_password is required to set a new password",
			})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(*req.CurrentPassword)); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "current password is incorrect",
			})
		}

		if len(*req.NewPassword) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
				Error: "new password must be at least 8 characters",
			})
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(*req.NewPassword), 12)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to hash password"})
		}
		user.PasswordHash = string(newHash)
		if h.SessionService != nil {
			// Revoke all other sessions after password change
			rawToken := c.Cookies("session")
			if rawToken == "" {
				authHeader := string(c.Request().Header.Peek("Authorization"))
				if strings.HasPrefix(authHeader, "Bearer ") {
					rawToken = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			currentHash := auth.HashToken(rawToken)
			_ = h.SessionService.RevokeUserSessions(userID, currentHash)
		}
	}

	if err := h.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to update user"})
	}

	return c.JSON(toUserResponse(&user))
}
