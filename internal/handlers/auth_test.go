package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/owen282000/life-dashboard-hub/internal/auth"
	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/middleware"
	"github.com/owen282000/life-dashboard-hub/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthTestApp(t *testing.T) (*fiber.App, *database.DB, *auth.SessionService) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	sessionService := auth.NewSessionService(db)
	oidcClient := &auth.OIDCClient{Enabled: false}
	authHandler := NewAuthHandler(db, sessionService, oidcClient, "http://localhost:8080")
	usersHandler := NewUsersHandler(db, sessionService)

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
	respPw, err := app.Test(reqPw, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("PATCH /users/me failed: %v", err)
	}
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

func setupAuthTestAppWithOIDC(t *testing.T, oidcClient *auth.OIDCClient) (*fiber.App, *database.DB, *auth.SessionService) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	sessionService := auth.NewSessionService(db)
	authHandler := NewAuthHandler(db, sessionService, oidcClient, "http://localhost:8080")
	usersHandler := NewUsersHandler(db, sessionService)

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

func TestOIDCAccountTakeoverPrevented(t *testing.T) {
	claims := &auth.OIDCClaims{
		Sub:   "attacker-sub-999",
		Email: "victim@example.com",
		Name:  "Attacker",
	}
	mockOIDC := &auth.OIDCClient{
		Enabled:   true,
		IssuerURL: "https://auth.example.com",
		ExchangeFunc: func(ctx context.Context, code string) (*auth.OIDCClaims, error) {
			return claims, nil
		},
	}

	app, db, _ := setupAuthTestAppWithOIDC(t, mockOIDC)

	// 1. Create a local user with email victim@example.com
	pwHash, _ := bcrypt.GenerateFromPassword([]byte("originalpassword"), 12)
	localUser := models.User{
		Email:        "victim@example.com",
		DisplayName:  "Victim User",
		PasswordHash: string(pwHash),
		AuthMethod:   "local",
		Status:       "active",
	}
	if err := db.Create(&localUser).Error; err != nil {
		t.Fatal(err)
	}

	// 2. Simulate OIDC callback with same email, different subject
	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback?state=xyz&code=validcode", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "xyz"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	// 3. Asserts response is 409 Conflict
	if resp.StatusCode != 409 {
		t.Fatalf("expected 409 Conflict, got %d (body: %s)", resp.StatusCode, string(bodyBytes))
	}

	// 4 & 5. Asserts user's PasswordHash is unchanged and AuthMethod is still "local"
	var checkUser models.User
	if err := db.First(&checkUser, localUser.ID).Error; err != nil {
		t.Fatal(err)
	}
	if checkUser.PasswordHash != string(pwHash) {
		t.Errorf("expected PasswordHash to remain unchanged, but it was modified")
	}
	if checkUser.AuthMethod != "local" {
		t.Errorf("expected AuthMethod 'local', got %s", checkUser.AuthMethod)
	}
	if checkUser.OIDCSubject != nil {
		t.Errorf("expected OIDCSubject to be nil, got %v", *checkUser.OIDCSubject)
	}
}

func TestOIDCCallbackStatusCheck(t *testing.T) {
	sub := "suspended-sub-123"
	mockOIDC := &auth.OIDCClient{
		Enabled:   true,
		IssuerURL: "https://auth.example.com",
		ExchangeFunc: func(ctx context.Context, code string) (*auth.OIDCClaims, error) {
			return &auth.OIDCClaims{
				Sub:   sub,
				Email: "suspended@example.com",
				Name:  "Suspended User",
			}, nil
		},
	}

	app, db, _ := setupAuthTestAppWithOIDC(t, mockOIDC)

	// Create suspended OIDC user
	issuer := "https://auth.example.com"
	suspendedUser := models.User{
		Email:       "suspended@example.com",
		DisplayName: "Suspended User",
		AuthMethod:  "oidc",
		OIDCSubject: &sub,
		OIDCIssuer:  &issuer,
		Status:      "suspended",
	}
	if err := db.Create(&suspendedUser).Error; err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback?state=xyz&code=validcode", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "xyz"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 Forbidden for suspended user, got %d (body: %s)", resp.StatusCode, string(bodyBytes))
	}
}

func TestSessionRevocationOnPasswordChange(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	// 1. Register a user
	regPayload := `{"email": "bob@example.com", "password": "password123", "display_name": "Bob"}`
	reqReg := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regPayload))
	reqReg.Header.Set("Content-Type", "application/json")
	respReg, _ := app.Test(reqReg)
	if respReg.StatusCode != 201 {
		t.Fatalf("expected 201 for register, got %d", respReg.StatusCode)
	}

	// Log in to get session 1
	loginPayload1 := `{"email": "bob@example.com", "password": "password123"}`
	reqLogin1 := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(loginPayload1))
	reqLogin1.Header.Set("Content-Type", "application/json")
	respLogin1, _ := app.Test(reqLogin1)
	if respLogin1.StatusCode != 200 {
		t.Fatalf("expected 200 for login 1, got %d", respLogin1.StatusCode)
	}
	session1 := extractCookie(respLogin1, "session")
	if session1 == "" {
		t.Fatal("missing session cookie 1")
	}

	// Log in again to get session 2
	loginPayload := `{"email": "bob@example.com", "password": "password123"}`
	reqLogin := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(loginPayload))
	reqLogin.Header.Set("Content-Type", "application/json")
	respLogin, _ := app.Test(reqLogin)
	if respLogin.StatusCode != 200 {
		t.Fatalf("expected 200 for login, got %d", respLogin.StatusCode)
	}
	session2 := extractCookie(respLogin, "session")
	if session2 == "" || session2 == session1 {
		t.Fatal("expected separate session cookie 2")
	}

	// Both sessions work initially
	reqMe1 := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMe1.AddCookie(&http.Cookie{Name: "session", Value: session1})
	respMe1, _ := app.Test(reqMe1)
	if respMe1.StatusCode != 200 {
		t.Fatalf("expected 200 for session 1 before pw change, got %d", respMe1.StatusCode)
	}

	reqMe2 := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMe2.AddCookie(&http.Cookie{Name: "session", Value: session2})
	respMe2, _ := app.Test(reqMe2)
	if respMe2.StatusCode != 200 {
		t.Fatalf("expected 200 for session 2 before pw change, got %d", respMe2.StatusCode)
	}

	// 2. Change password using session 1
	pwChangePayload := `{"current_password": "password123", "new_password": "newpassword456"}`
	reqPw := httptest.NewRequest("PATCH", "/api/v1/users/me", strings.NewReader(pwChangePayload))
	reqPw.Header.Set("Content-Type", "application/json")
	reqPw.AddCookie(&http.Cookie{Name: "session", Value: session1})
	respPw, err := app.Test(reqPw, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("PATCH /users/me failed: %v", err)
	}
	if respPw.StatusCode != 200 {
		t.Fatalf("expected 200 for valid password change, got %d", respPw.StatusCode)
	}

	// 3. Asserts session 2 is revoked (GET /api/v1/users/me with session 2 returns 401)
	reqMe2After := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMe2After.AddCookie(&http.Cookie{Name: "session", Value: session2})
	respMe2After, _ := app.Test(reqMe2After)
	if respMe2After.StatusCode != 401 {
		t.Fatalf("expected 401 for session 2 after password change, got %d", respMe2After.StatusCode)
	}

	// 4. Asserts session 1 still works
	reqMe1After := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMe1After.AddCookie(&http.Cookie{Name: "session", Value: session1})
	respMe1After, _ := app.Test(reqMe1After)
	if respMe1After.StatusCode != 200 {
		t.Fatalf("expected 200 for session 1 after password change, got %d", respMe1After.StatusCode)
	}
}

func TestLogoutBearerToken(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	// Register
	regPayload := `{"email": "bearer@example.com", "password": "password123", "display_name": "Bearer User"}`
	reqReg := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(regPayload))
	reqReg.Header.Set("Content-Type", "application/json")
	respReg, _ := app.Test(reqReg)
	if respReg.StatusCode != 201 {
		t.Fatalf("expected 201 for register, got %d", respReg.StatusCode)
	}

	loginPayload := `{"email": "bearer@example.com", "password": "password123"}`
	reqLogin := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(loginPayload))
	reqLogin.Header.Set("Content-Type", "application/json")
	respLogin, _ := app.Test(reqLogin)
	if respLogin.StatusCode != 200 {
		t.Fatalf("expected 200 for login, got %d", respLogin.StatusCode)
	}
	token := extractCookie(respLogin, "session")
	if token == "" {
		t.Fatal("missing session token")
	}
	// Access with Bearer token works
	reqMe := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+token)
	respMe, _ := app.Test(reqMe)
	if respMe.StatusCode != 200 {
		t.Fatalf("expected 200 with Bearer auth, got %d", respMe.StatusCode)
	}

	// Logout with Bearer token
	reqLogout := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	reqLogout.Header.Set("Authorization", "Bearer "+token)
	respLogout, _ := app.Test(reqLogout)
	if respLogout.StatusCode != 200 {
		t.Fatalf("expected 200 with Bearer logout, got %d", respLogout.StatusCode)
	}

	// Verify token is revoked
	reqMeAfter := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	reqMeAfter.Header.Set("Authorization", "Bearer "+token)
	respMeAfter, _ := app.Test(reqMeAfter)
	if respMeAfter.StatusCode != 401 {
		t.Fatalf("expected 401 after Bearer logout, got %d", respMeAfter.StatusCode)
	}
}
