package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/owen282000/life-dashboard-hub/internal/auth"
	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/middleware"
	"github.com/owen282000/life-dashboard-hub/internal/models"
)

func setupFamilyTestApp(t *testing.T) (*fiber.App, *database.DB, *auth.SessionService, *models.User, *models.User, string, string) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	sessionService := auth.NewSessionService(db)

	parentUser := models.User{
		Email:       "parent@example.com",
		DisplayName: "Parent User",
		Status:      "active",
		AuthMethod:  "local",
	}
	adultUser := models.User{
		Email:       "adult@example.com",
		DisplayName: "Adult User",
		Status:      "active",
		AuthMethod:  "local",
	}
	_ = db.Create(&parentUser)
	_ = db.Create(&adultUser)

	parentToken, _, _ := sessionService.CreateSession(parentUser.ID)
	adultToken, _, _ := sessionService.CreateSession(adultUser.ID)

	app := fiber.New()
	api := app.Group("/api/v1")
	protected := api.Group("", middleware.RequireAuth(sessionService))

	reportsHandler := NewReportsHandler(db)
	familiesHandler := NewFamiliesHandler(db, reportsHandler)

	familiesGroup := protected.Group("/families")
	familiesGroup.Post("", familiesHandler.CreateFamily)
	familiesGroup.Get("", familiesHandler.ListFamilies)
	familiesGroup.Get("/:id", familiesHandler.GetFamily)
	familiesGroup.Post("/:id/invitations", familiesHandler.InviteMember)
	familiesGroup.Post("/:id/children", familiesHandler.CreateChildAccount)
	familiesGroup.Patch("/:familyId/members/:userId/sharing", familiesHandler.UpdateSharing)
	familiesGroup.Post("/:familyId/leave", familiesHandler.LeaveFamily)
	familiesGroup.Delete("/:familyId/members/:userId", familiesHandler.RemoveMember)
	familiesGroup.Get("/:familyId/members/:userId/reports/screen-time/daily", familiesHandler.GetMemberScreenTimeDaily)
	familiesGroup.Get("/:familyId/members/:userId/reports/screen-time/apps", familiesHandler.GetMemberScreenTimeApps)
	familiesGroup.Get("/:familyId/members/:userId/reports/health/daily", familiesHandler.GetMemberHealthDaily)

	protected.Post("/invitations/:token/accept", familiesHandler.AcceptInvitation)

	return app, db, sessionService, &parentUser, &adultUser, parentToken, adultToken
}

func TestFamilyCreationInvitationAndSharing(t *testing.T) {
	app, db, _, parent, adult, parentCookie, adultCookie := setupFamilyTestApp(t)

	// 1. Parent creates family
	createReq := httptest.NewRequest("POST", "/api/v1/families", strings.NewReader(`{"name": "The Smiths"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != 201 {
		b, _ := io.ReadAll(createResp.Body)
		t.Fatalf("expected 201 on create family, got %d: %s", createResp.StatusCode, string(b))
	}

	var famResp FamilyResponse
	_ = json.NewDecoder(createResp.Body).Decode(&famResp)
	familyID := famResp.ID
	if famResp.Name != "The Smiths" || famResp.Role != "admin" {
		t.Errorf("unexpected family resp: %+v", famResp)
	}

	// 2. Parent invites Adult member
	inviteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/families/%d/invitations", familyID), strings.NewReader(`{"email": "adult@example.com", "role": "adult"}`))
	inviteReq.Header.Set("Content-Type", "application/json")
	inviteReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	inviteResp, _ := app.Test(inviteReq)
	if inviteResp.StatusCode != 201 {
		b, _ := io.ReadAll(inviteResp.Body)
		t.Fatalf("expected 201 for invite, got %d: %s", inviteResp.StatusCode, string(b))
	}
	var invResp InvitationResponse
	_ = json.NewDecoder(inviteResp.Body).Decode(&invResp)
	inviteToken := invResp.InviteToken
	if inviteToken == "" {
		t.Fatal("expected invite token")
	}

	// 3. Adult accepts invitation
	acceptReq := httptest.NewRequest("POST", "/api/v1/invitations/"+inviteToken+"/accept", nil)
	acceptReq.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	acceptResp, _ := app.Test(acceptReq)
	if acceptResp.StatusCode != 200 {
		t.Fatalf("expected 200 on accept invite, got %d", acceptResp.StatusCode)
	}

	// 4. Seed data for Adult user
	adultDev := models.Device{
		UserID:      adult.ID,
		Label:       "adult-phone",
		DisplayName: "Adult Phone",
		TokenHash:   "hash-adult",
		Status:      "active",
	}
	_ = db.Create(&adultDev)

	dayStr := "2026-10-11"
	stData, _ := json.Marshal(map[string]interface{}{
		"total_screen_time_minutes": 250,
		"apps": []map[string]interface{}{
			{"package_name": "com.work", "app_name": "Work App", "minutes": 200},
		},
	})
	seq := int64(1)
	_ = db.Create(&models.Record{
		DeviceID: adultDev.ID,
		Type:     "screen_time",
		UUID:     "adult-st-uuid",
		Day:      &dayStr,
		Data:     string(stData),
		Seq:      &seq,
	})

	// 5. Parent attempts to view adult's reports with sharing="none" -> 403 Forbidden
	viewDailyURL := fmt.Sprintf("/api/v1/families/%d/members/%d/reports/screen-time/daily", familyID, adult.ID)
	viewReq := httptest.NewRequest("GET", viewDailyURL, nil)
	viewReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	viewResp, _ := app.Test(viewReq)
	if viewResp.StatusCode != 403 {
		t.Errorf("expected 403 when sharing=none, got %d", viewResp.StatusCode)
	}

	// 6. Adult updates sharing to "summary"
	shareUpdateURL := fmt.Sprintf("/api/v1/families/%d/members/%d/sharing", familyID, adult.ID)
	shareReq := httptest.NewRequest("PATCH", shareUpdateURL, strings.NewReader(`{"sharing": "summary"}`))
	shareReq.Header.Set("Content-Type", "application/json")
	shareReq.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	shareResp, _ := app.Test(shareReq)
	if shareResp.StatusCode != 200 {
		t.Fatalf("expected 200 on update sharing, got %d", shareResp.StatusCode)
	}

	// Parent views daily totals -> 200 OK
	viewRespSummary, _ := app.Test(viewReq)
	if viewRespSummary.StatusCode != 200 {
		t.Errorf("expected 200 for summary sharing, got %d", viewRespSummary.StatusCode)
	}

	// But parent views apps breakdown -> 403 Forbidden
	viewAppsURL := fmt.Sprintf("/api/v1/families/%d/members/%d/reports/screen-time/apps", familyID, adult.ID)
	viewAppsReq := httptest.NewRequest("GET", viewAppsURL, nil)
	viewAppsReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	viewAppsResp, _ := app.Test(viewAppsReq)
	if viewAppsResp.StatusCode != 403 {
		t.Errorf("expected 403 for apps breakdown when sharing=summary, got %d", viewAppsResp.StatusCode)
	}

	// 7. Adult updates sharing to "detailed"
	shareReq2 := httptest.NewRequest("PATCH", shareUpdateURL, strings.NewReader(`{"sharing": "detailed"}`))
	shareReq2.Header.Set("Content-Type", "application/json")
	shareReq2.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	_, _ = app.Test(shareReq2)

	// Parent views apps breakdown -> 200 OK
	viewAppsResp2, _ := app.Test(viewAppsReq)
	if viewAppsResp2.StatusCode != 200 {
		t.Errorf("expected 200 for apps breakdown when sharing=detailed, got %d", viewAppsResp2.StatusCode)
	}

	// 8. Create child account
	childReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/families/%d/children", familyID), strings.NewReader(`{"display_name": "Kiddo", "password": "kidpassword123"}`))
	childReq.Header.Set("Content-Type", "application/json")
	childReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	childResp, _ := app.Test(childReq)
	if childResp.StatusCode != 201 {
		b, _ := io.ReadAll(childResp.Body)
		t.Fatalf("expected 201 on create child account, got %d: %s", childResp.StatusCode, string(b))
	}
	var childUser UserResponse
	_ = json.NewDecoder(childResp.Body).Decode(&childUser)

	// Seed data for child
	childDev := models.Device{
		UserID:      childUser.ID,
		Label:       "child-tablet",
		DisplayName: "Kiddo Tablet",
		TokenHash:   "hash-child",
		Status:      "active",
	}
	_ = db.Create(&childDev)
	_ = db.Create(&models.Record{
		DeviceID: childDev.ID,
		Type:     "screen_time",
		UUID:     "child-st-uuid",
		Day:      &dayStr,
		Data:     string(stData),
		Seq:      &seq,
	})

	// Parent can view child's detailed reports automatically without requiring explicit permission
	viewChildAppsURL := fmt.Sprintf("/api/v1/families/%d/members/%d/reports/screen-time/apps", familyID, childUser.ID)
	viewChildReq := httptest.NewRequest("GET", viewChildAppsURL, nil)
	viewChildReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	viewChildResp, _ := app.Test(viewChildReq)
	if viewChildResp.StatusCode != 200 {
		t.Errorf("expected 200 for parent viewing child reports, got %d", viewChildResp.StatusCode)
	}

	_ = parent
}
