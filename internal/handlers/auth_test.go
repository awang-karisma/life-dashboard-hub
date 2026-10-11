package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/awang-karisma/life-dashboard-hub/internal/auth"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/middleware"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
)

func setupAuthTestApp(t *testing.T) (*fiber.App, *database.DB, *auth.SessionService) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	sessionService := auth.NewSessionService(db)
	oidcClient := &auth.OIDCClient{Enabled: false}
	authHandler := NewAuthHandler(db, sessionService, oidcClient, "http://localhost:8080")
	usersHandler := NewUsersHandler(db)

	app := fiber.New()
	api := app.Group("/api/v1")

	authGroup := api.Group("/auth")
	authGroup.Get("/info", authHandler.GetAuthInfo)
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/logout", authHandler.Logout)
	authGroup.Get("/oidc/login", authHandler.OIDCLogin)
	authGroup.Get("/oidc/callback", authHandler.OIDCCallback)

	protected := api.Group("", middleware.RequireAuth(sessionService))
	protected.Get("/users/me", usersHandler.GetCurrentUser)
	protected.Patch("/users/me", usersHandler.UpdateCurrentUser)

	return app, db, sessionService
}

func extractCookie(resp *http.Response, name string) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func TestAuthInfo(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/auth/info", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var info AuthInfoResponse
	_ = json.NewDecoder(resp.Body).Decode(&info)
	if !info.Local {
		t.Error("expected local=true")
	}
	if info.OIDC.Enabled {
		t.Error("expected oidc.enabled=false")
	}

	// Verify OIDC login returns 404 when disabled
	reqOIDC := httptest.NewRequest("GET", "/api/v1/auth/oidc/login", nil)
	respOIDC, _ := app.Test(reqOIDC)
	if respOIDC.StatusCode != 404 {
		t.Errorf("expected 404 for disabled OIDC login, got %d", respOIDC.StatusCode)
	}
}

func TestLocalAuthAndUserProfileFlow(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	// 1. Register user
	regPayload := `{"email": "alice@example.com", "password": "password123", "display_name": "Alice"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regPayload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
	}

	var userResp UserResponse
	_ = json.NewDecoder(resp.Body).Decode(&userResp)
	if userResp.Email != "alice@example.com" || userResp.DisplayName != "Alice" {
		t.Fatalf("unexpected user response: %+v", userResp)
	}

	// Duplicate registration -> 409
	reqDup := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regPayload))
	reqDup.Header.Set("Content-Type", "application/json")
	respDup, _ := app.Test(reqDup)
	if respDup.StatusCode != 409 {
		t.Errorf("expected 409 for duplicate email, got %d", respDup.StatusCode)
	}

	// Short password -> 400
	badPw := `{"email": "bob@example.com", "password": "short"}`
	reqShort := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(badPw))
	reqShort.Header.Set("Content-Type", "application/json")
	respShort, _ := app.Test(reqShort)
	if respShort.StatusCode != 400 {
		t.Errorf("expected 400 for short password, got %d", respShort.StatusCode)
	}

	// 2. Login
	loginPayload := `{"email": "alice@example.com", "password": "password123"}`
	reqLogin := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(loginPayload))
	reqLogin.Header.Set("Content-Type", "application/json")
	respLogin, err := app.Test(reqLogin)
	if err != nil {
		t.Fatal(err)
	}
	defer respLogin.Body.Close()

	if respLogin.StatusCode != 200 {
		t.Fatalf("expected 200 for login, got %d", respLogin.StatusCode)
	}

	sessionCookie := extractCookie(respLogin, "session")
	if sessionCookie == "" {
		t.Fatal("expected session cookie to be set")
	}

	// Invalid password -> 401
	badLogin := `{"email": "alice@example.com", "password": "wrongpassword"}`
	reqBadLogin := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(badLogin))
	reqBadLogin.Header.Set("Content-Type", "application/json")
	respBadLogin, _ := app.Test(reqBadLogin)
	if respBadLogin.StatusCode != 401 {
		t.Errorf("expected 401 for wrong password, got %d", respBadLogin.StatusCode)
	}

	// 3. Protected endpoint GET /users/me
	// Without cookie -> 401
	reqMeNoAuth := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	respMeNoAuth, _ := app.Test(reqMeNoAuth)
	if respMeNoAuth.StatusCode != 401 {
		t.Errorf("expected 401 without auth, got %d", respMeNoAuth.StatusCode)
	}

	// With cookie -> 200
	reqMe := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMe.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respMe, err := app.Test(reqMe)
	if err != nil {
		t.Fatal(err)
	}
	defer respMe.Body.Close()

	if respMe.StatusCode != 200 {
		t.Fatalf("expected 200 for authenticated /users/me, got %d", respMe.StatusCode)
	}

	var meResp UserResponse
	_ = json.NewDecoder(respMe.Body).Decode(&meResp)
	if meResp.Email != "alice@example.com" || meResp.AuthMethod != "local" {
		t.Errorf("unexpected profile: %+v", meResp)
	}

	// 4. Update profile PATCH /users/me
	updatePayload := `{"display_name": "Alice Smith", "timezone": "America/New_York"}`
	reqUpdate := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(updatePayload))
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respUpdate, err := app.Test(reqUpdate)
	if err != nil {
		t.Fatal(err)
	}
	defer respUpdate.Body.Close()

	if respUpdate.StatusCode != 200 {
		t.Fatalf("expected 200 for update, got %d", respUpdate.StatusCode)
	}
	var updatedUser UserResponse
	_ = json.NewDecoder(respUpdate.Body).Decode(&updatedUser)
	if updatedUser.DisplayName != "Alice Smith" || updatedUser.Timezone != "America/New_York" {
		t.Errorf("update didn't reflect: %+v", updatedUser)
	}

	// 5. Change password
	pwChangePayload := `{"current_password": "password123", "new_password": "newpassword456"}`
	reqPw := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(pwChangePayload))
	reqPw.Header.Set("Content-Type", "application/json")
	reqPw.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respPw, _ := app.Test(reqPw)
	if respPw.StatusCode != 200 {
		t.Fatalf("expected 200 for valid password change, got %d", respPw.StatusCode)
	}

	// 6. Logout
	reqLogout := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	reqLogout.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respLogout, _ := app.Test(reqLogout)
	if respLogout.StatusCode != 200 {
		t.Fatalf("expected 200 for logout, got %d", respLogout.StatusCode)
	}

	// Subsequent /users/me with revoked session -> 401
	reqMeRevoked := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMeRevoked.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respMeRevoked, _ := app.Test(reqMeRevoked)
	if respMeRevoked.StatusCode != 401 {
		t.Errorf("expected 401 with revoked session, got %d", respMeRevoked.StatusCode)
	}

	// Login with new password works
	reqLoginNew := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader([]byte(`{"email": "alice@example.com", "password": "newpassword456"}`)))
	reqLoginNew.Header.Set("Content-Type", "application/json")
	respLoginNew, _ := app.Test(reqLoginNew)
	if respLoginNew.StatusCode != 200 {
		t.Fatalf("expected 200 login with new password, got %d", respLoginNew.StatusCode)
	}
}

func TestOIDCUserRestrictions(t *testing.T) {
	app, db, sessionService := setupAuthTestApp(t)

	// Create an OIDC-authenticated user
	sub := "oidc-sub-12345"
	issuer := "https://auth.example.com"
	oidcUser := models.User{
		Email:       "oidcuser@example.com",
		DisplayName: "SSO User",
		AuthMethod:  "oidc",
		OIDCSubject: &sub,
		OIDCIssuer:  &issuer,
		Status:      "active",
	}
	if err := db.Create(&oidcUser).Error; err != nil {
		t.Fatal(err)
	}

	// 1. Attempt local login with OIDC account -> 400 "This account uses SSO login"
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email": "oidcuser@example.com", "password": "anypassword"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, _ := app.Test(loginReq)
	if loginResp.StatusCode != 400 {
		t.Errorf("expected 400 for local login on OIDC user, got %d", loginResp.StatusCode)
	}

	// 2. Active session for OIDC user
	rawToken, _, err := sessionService.CreateSession(oidcUser.ID)
	if err != nil {
		t.Fatal(err)
	}

	// 3. OIDC user attempts to set password via PATCH /users/me -> 400
	pwReq := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(`{"current_password": "foo", "new_password": "newpassword123"}`))
	pwReq.Header.Set("Content-Type", "application/json")
	pwReq.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	pwResp, _ := app.Test(pwReq)
	if pwResp.StatusCode != 400 {
		t.Errorf("expected 400 when OIDC user tries to set password, got %d", pwResp.StatusCode)
	}
}
