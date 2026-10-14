package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/awang-karisma/life-dashboard-hub/internal/auth"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB             *database.DB
	SessionService *auth.SessionService
	OIDCClient     *auth.OIDCClient
	PublicURL      string
}

func NewAuthHandler(db *database.DB, sessionService *auth.SessionService, oidcClient *auth.OIDCClient, publicURL string) *AuthHandler {
	return &AuthHandler{
		DB:             db,
		SessionService: sessionService,
		OIDCClient:     oidcClient,
		PublicURL:      publicURL,
	}
}

type AuthInfoResponse struct {
	Local bool             `json:"local"`
	OIDC  OIDCInfoResponse `json:"oidc"`
}

type OIDCInfoResponse struct {
	Enabled     bool   `json:"enabled"`
	DisplayName string `json:"display_name,omitempty"`
	LoginURL    string `json:"login_url,omitempty"`
}

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID          uint      `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Timezone    string    `json:"timezone"`
	Status      string    `json:"status"`
	AuthMethod  string    `json:"auth_method"`
	CreatedAt   time.Time `json:"created_at"`
}

func toUserResponse(u *models.User) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Timezone:    u.Timezone,
		Status:      u.Status,
		AuthMethod:  u.AuthMethod,
		CreatedAt:   u.CreatedAt,
	}
}

// GetAuthInfo godoc
// @Summary      Get available authentication methods
// @Description  Returns local and OIDC configuration info for the login view
// @Tags         auth
// @Produce      json
// @Success      200 {object} AuthInfoResponse
// @Router       /auth/info [get]
func (h *AuthHandler) GetAuthInfo(c fiber.Ctx) error {
	resp := AuthInfoResponse{
		Local: true,
		OIDC: OIDCInfoResponse{
			Enabled: false,
		},
	}

	if h.OIDCClient != nil && h.OIDCClient.Enabled {
		resp.OIDC.Enabled = true
		resp.OIDC.DisplayName = h.OIDCClient.DisplayName
		resp.OIDC.LoginURL = "/api/v1/auth/oidc/login"
	}

	return c.JSON(resp)
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a local account with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterRequest true "Registration payload"
// @Success      201 {object} UserResponse
// @Failure      400 {object} ErrorResponse
// @Failure      409 {object} ErrorResponse "Email already registered"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req RegisterRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "valid email address is required"})
	}

	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "password must be at least 8 characters"})
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		parts := strings.Split(req.Email, "@")
		displayName = parts[0]
	}

	// Check if email already exists
	var existing models.User
	if err := h.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(ErrorResponse{Error: "email already registered"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to hash password"})
	}

	user := models.User{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		DisplayName:  displayName,
		Timezone:     "UTC",
		Status:       "active",
		AuthMethod:   "local",
	}

	if err := h.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to create user account"})
	}

	return c.Status(fiber.StatusCreated).JSON(toUserResponse(&user))
}

// Login godoc
// @Summary      Login with local credentials
// @Description  Authenticate with email and password to receive a session cookie
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "Login payload"
// @Success      200 {object} UserResponse
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse "Invalid credentials"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "invalid email or password"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "internal error"})
	}

	if user.AuthMethod == "oidc" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "This account uses SSO login"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(ErrorResponse{Error: "invalid email or password"})
	}

	if user.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "account is " + user.Status})
	}

	rawToken, _, err := h.SessionService.CreateSession(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to create session"})
	}

	isSecure := strings.HasPrefix(strings.ToLower(h.PublicURL), "https://")
	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    rawToken,
		Path:     "/",
		MaxAge:   int(auth.SessionDuration.Seconds()),
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	return c.JSON(toUserResponse(&user))
}

// Logout godoc
// @Summary      Logout current session
// @Description  Revokes session and clears session cookie
// @Tags         auth
// @Produce      json
// @Success      200 {object} MessageResponse
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	rawToken := c.Cookies("session")
	if rawToken == "" {
		authHeader := string(c.Request().Header.Peek("Authorization"))
		if strings.HasPrefix(authHeader, "Bearer ") {
			rawToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if rawToken != "" {
		_ = h.SessionService.RevokeSession(rawToken)
	}

	isSecure := strings.HasPrefix(strings.ToLower(h.PublicURL), "https://")
	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	return c.JSON(MessageResponse{Message: "logged out"})
}

// OIDCLogin godoc
// @Summary      Initiate OIDC SSO login
// @Description  Redirects user to OIDC identity provider
// @Tags         auth
// @Success      307
// @Failure      404 {object} ErrorResponse "OIDC not enabled"
// @Router       /auth/oidc/login [get]
func (h *AuthHandler) OIDCLogin(c fiber.Ctx) error {
	if h.OIDCClient == nil || !h.OIDCClient.Enabled {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "OIDC login is not enabled"})
	}

	// Generate random state
	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to generate state"})
	}
	state := hex.EncodeToString(stateBytes)

	isSecure := strings.HasPrefix(strings.ToLower(h.PublicURL), "https://")
	c.Cookie(&fiber.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/api/v1/auth/oidc",
		MaxAge:   600, // 10 minutes
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	authURL := h.OIDCClient.AuthCodeURL(state)
	return c.Redirect().To(authURL)
}

// OIDCCallback godoc
// @Summary      OIDC SSO Callback
// @Description  Processes callback from OIDC identity provider
// @Tags         auth
// @Success      302
// @Failure      400 {object} ErrorResponse "Invalid state or code"
// @Router       /auth/oidc/callback [get]
func (h *AuthHandler) OIDCCallback(c fiber.Ctx) error {
	if h.OIDCClient == nil || !h.OIDCClient.Enabled {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "OIDC is not enabled"})
	}

	state := c.Query("state")
	cookieState := c.Cookies("oidc_state")

	isSecure := strings.HasPrefix(strings.ToLower(h.PublicURL), "https://")
	// Clear oidc_state cookie
	c.Cookie(&fiber.Cookie{
		Name:     "oidc_state",
		Value:    "",
		Path:     "/api/v1/auth/oidc",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	if state == "" || cookieState == "" || state != cookieState {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid or expired OIDC state parameter"})
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "missing authorization code"})
	}

	claims, err := h.OIDCClient.ExchangeAndVerify(c.Context(), code)
	if err != nil {
		slog.Error("OIDC code exchange failed", "error", err)
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "failed to verify OIDC token: " + err.Error()})
	}

	claims.Email = strings.ToLower(strings.TrimSpace(claims.Email))
	if claims.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "OIDC provider did not return an email claim"})
	}

	// Find or create user
	var user models.User
	// 1. Look up by (oidc_issuer, oidc_subject)
	err = h.DB.Where("oidc_issuer = ? AND oidc_subject = ?", h.OIDCClient.IssuerURL, claims.Sub).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 2. Look up by email
			errEmail := h.DB.Where("email = ?", claims.Email).First(&user).Error
			if errEmail == nil {
				// Email already used by a local account — refuse auto-link
				return c.Status(fiber.StatusConflict).JSON(ErrorResponse{
					Error: "an account with this email already exists — log in with your password first, then link SSO from settings",
				})
			}

			// Create new user
			displayName := strings.TrimSpace(claims.Name)
			if displayName == "" {
				parts := strings.Split(claims.Email, "@")
				displayName = parts[0]
			}
			user = models.User{
				Email:       claims.Email,
				DisplayName: displayName,
				Timezone:    "UTC",
				Status:      "active",
				AuthMethod:  "oidc",
				OIDCSubject: &claims.Sub,
				OIDCIssuer:  &h.OIDCClient.IssuerURL,
			}
			if err := h.DB.Create(&user).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to create user account"})
			}
		} else {
			return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "database error"})
		}
	}

	if user.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(ErrorResponse{Error: "account is " + user.Status})
	}

	rawToken, _, err := h.SessionService.CreateSession(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to create session"})
	}

	c.Cookie(&fiber.Cookie{
		Name:     "session",
		Value:    rawToken,
		Path:     "/",
		MaxAge:   int(auth.SessionDuration.Seconds()),
		HTTPOnly: true,
		Secure:   isSecure,
		SameSite: "Lax",
	})

	return c.Redirect().To("/?oidc_login=success")
}
