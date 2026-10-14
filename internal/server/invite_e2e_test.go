package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/awang-karisma/life-dashboard-hub/internal/auth"
	"github.com/awang-karisma/life-dashboard-hub/internal/config"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/gofiber/fiber/v3"
	"github.com/awang-karisma/life-dashboard-hub/internal/handlers"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
)

func TestFamilyInviteLinkFlowE2E(t *testing.T) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	cfg := &config.Config{
		PublicURL:    "http://localhost:8080",
		MaxBodyBytes: 10 * 1024 * 1024,
	}

	srv := New(cfg, db)

	// Helper for requests
	doReq := func(method, path string, body any, cookie *http.Cookie, target any) *http.Response {
		var reqBody *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			reqBody = bytes.NewReader(b)
		} else {
			reqBody = bytes.NewReader([]byte{})
		}

		req := httptest.NewRequest(method, path, reqBody)
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}

		resp, err := srv.App.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
		if err != nil {
			t.Fatalf("%s %s request failed: %v", method, path, err)
		}

		if target != nil {
			_ = json.NewDecoder(resp.Body).Decode(target)
		}
		return resp
	}

	// 1. Register family admin
	regResp := doReq("POST", "/api/v1/auth/register", map[string]string{
		"email":        "admin@example.com",
		"password":     "password123",
		"display_name": "Family Admin",
	}, nil, nil)
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to register admin: got status %d", regResp.StatusCode)
	}

	// 2. Log in admin to obtain session cookie
	loginResp := doReq("POST", "/api/v1/auth/login", map[string]string{
		"email":    "admin@example.com",
		"password": "password123",
	}, nil, nil)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to login admin: got status %d", loginResp.StatusCode)
	}

	var adminCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "session" {
			adminCookie = c
			break
		}
	}
	if adminCookie == nil {
		t.Fatal("expected session cookie from login")
	}

	// 3. Admin creates family
	var famResp handlers.FamilyResponse
	createFamResp := doReq("POST", "/api/v1/families", map[string]string{
		"name": "The Incredibles",
	}, adminCookie, &famResp)
	if createFamResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create family: got status %d", createFamResp.StatusCode)
	}
	if famResp.Name != "The Incredibles" {
		t.Fatalf("expected family name 'The Incredibles', got %q", famResp.Name)
	}
	familyID := famResp.ID

	// 4. Admin invites member
	var invResp handlers.InvitationResponse
	inviteResp := doReq("POST", fmt.Sprintf("/api/v1/families/%d/invitations", familyID), map[string]string{
		"email": "member@example.com",
		"role":  "adult",
	}, adminCookie, &invResp)
	if inviteResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to invite member: got status %d", inviteResp.StatusCode)
	}
	rawToken := invResp.InviteToken
	if rawToken == "" {
		t.Fatalf("expected non-empty invite_token")
	}

	// 5. Unauthenticated user previews invitation (Step 1 public preview endpoint)
	var previewData handlers.InvitationPreviewResponse
	previewResp := doReq("GET", fmt.Sprintf("/api/v1/invitations/%s/preview", rawToken), nil, nil, &previewData)
	if previewResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for preview, got %d", previewResp.StatusCode)
	}
	if previewData.FamilyName != "The Incredibles" {
		t.Errorf("expected family_name 'The Incredibles', got %q", previewData.FamilyName)
	}
	if previewData.InvitedEmail != "member@example.com" {
		t.Errorf("expected invited_email 'member@example.com', got %q", previewData.InvitedEmail)
	}
	if previewData.Role != "adult" {
		t.Errorf("expected role 'adult', got %q", previewData.Role)
	}
	if previewData.ExpiresAt.IsZero() {
		t.Errorf("expected non-zero expires_at")
	}

	// 6. Preview non-existent token -> 404
	notFoundResp := doReq("GET", "/api/v1/invitations/invalid-token-12345/preview", nil, nil, nil)
	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for invalid token, got %d", notFoundResp.StatusCode)
	}

	sessionService := auth.NewSessionService(db)

	// 7. Create intruder user with different email
	intruder := models.User{
		Email:       "intruder@example.com",
		DisplayName: "Intruder",
		Status:      "active",
		AuthMethod:  "local",
	}
	if err := db.Create(&intruder).Error; err != nil {
		t.Fatalf("failed to create intruder: %v", err)
	}
	intruderCookieVal, _, err := sessionService.CreateSession(intruder.ID)
	if err != nil {
		t.Fatalf("failed to create intruder session: %v", err)
	}
	intruderCookie := &http.Cookie{Name: "session", Value: intruderCookieVal}

	// 8. Intruder attempts to accept invitation -> 403 Forbidden
	var errResp handlers.ErrorResponse
	intruderAcceptResp := doReq("POST", fmt.Sprintf("/api/v1/invitations/%s/accept", rawToken), nil, intruderCookie, &errResp)
	if intruderAcceptResp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 when intruder accepts, got %d (%v)", intruderAcceptResp.StatusCode, errResp)
	}

	// 9. Create intended user with "member@example.com"
	member := models.User{
		Email:       "member@example.com",
		DisplayName: "Intended Member",
		Status:      "active",
		AuthMethod:  "local",
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("failed to create member: %v", err)
	}
	memberCookieVal, _, err := sessionService.CreateSession(member.ID)
	if err != nil {
		t.Fatalf("failed to create member session: %v", err)
	}
	memberCookie := &http.Cookie{Name: "session", Value: memberCookieVal}

	// 10. Intended user accepts invitation -> 200 OK
	var msgResp handlers.MessageResponse
	memberAcceptResp := doReq("POST", fmt.Sprintf("/api/v1/invitations/%s/accept", rawToken), nil, memberCookie, &msgResp)
	if memberAcceptResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when intended user accepts, got %d (%v)", memberAcceptResp.StatusCode, msgResp)
	}

	// 11. Intended user is now a member of the family
	var memberFams []handlers.FamilyResponse
	listFamResp := doReq("GET", "/api/v1/families", nil, memberCookie, &memberFams)
	if listFamResp.StatusCode != http.StatusOK {
		t.Fatalf("failed to list families: got %d", listFamResp.StatusCode)
	}
	if len(memberFams) != 1 || memberFams[0].ID != familyID {
		t.Fatalf("expected member to belong to family %d, got %+v", familyID, memberFams)
	}

	// 12. Preview of already accepted invitation -> 410 Gone
	usedPreviewResp := doReq("GET", fmt.Sprintf("/api/v1/invitations/%s/preview", rawToken), nil, nil, nil)
	if usedPreviewResp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 for used token preview, got %d", usedPreviewResp.StatusCode)
	}

	// 13. Expired invitation preview -> 410 Gone
	var secondInv handlers.InvitationResponse
	secondInviteResp := doReq("POST", fmt.Sprintf("/api/v1/families/%d/invitations", familyID), map[string]string{
		"email": "another@example.com",
		"role":  "child",
	}, adminCookie, &secondInv)
	if secondInviteResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create second invite: got %d", secondInviteResp.StatusCode)
	}
	secondToken := secondInv.InviteToken

	hHash := sha256.New()
	hHash.Write([]byte(secondToken))
	secondHash := hex.EncodeToString(hHash.Sum(nil))

	var inv models.Invitation
	if err := db.Where("token_hash = ?", secondHash).First(&inv).Error; err != nil {
		t.Fatalf("failed to find invitation in db: %v", err)
	}
	inv.ExpiresAt = time.Now().UTC().Add(-10 * time.Minute)
	db.Save(&inv)

	expiredPreviewResp := doReq("GET", fmt.Sprintf("/api/v1/invitations/%s/preview", secondToken), nil, nil, nil)
	if expiredPreviewResp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 for expired token preview, got %d", expiredPreviewResp.StatusCode)
	}
}
