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
	"github.com/awang-karisma/life-dashboard-hub/internal/auth"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/middleware"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
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

	reportsHandler := NewReportsHandler(db, nil)
	familiesHandler := NewFamiliesHandler(db, reportsHandler)

	familiesGroup := protected.Group("/families")
	familiesGroup.Post("", familiesHandler.CreateFamily)
	familiesGroup.Get("", familiesHandler.ListFamilies)
	familiesGroup.Get("/:id", familiesHandler.GetFamily)
	familiesGroup.Patch("/:id", familiesHandler.UpdateFamily)
	familiesGroup.Delete("/:id", familiesHandler.DeleteFamily)
	familiesGroup.Get("/:id/settings", familiesHandler.GetFamilySettings)
	familiesGroup.Patch("/:id/settings", familiesHandler.UpdateFamilySettings)
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
func TestSingleFamilyConstraint(t *testing.T) {
	app, db, sessionService, parent, adult, parentCookie, adultCookie := setupFamilyTestApp(t)
	_ = db
	_ = parent
	_ = adult

	// 1. Parent creates family 1
	createReq := httptest.NewRequest("POST", "/api/v1/families", strings.NewReader(`{"name": "Family One"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}
	var famResp FamilyResponse
	_ = json.NewDecoder(createResp.Body).Decode(&famResp)

	// 2. Parent attempts to create another family -> 409
	createReq2 := httptest.NewRequest("POST", "/api/v1/families", strings.NewReader(`{"name": "Family Two"}`))
	createReq2.Header.Set("Content-Type", "application/json")
	createReq2.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	createResp2, err := app.Test(createReq2)
	if err != nil {
		t.Fatal(err)
	}
	defer createResp2.Body.Close()
	if createResp2.StatusCode != 409 {
		b, _ := io.ReadAll(createResp2.Body)
		t.Fatalf("expected 409 when creating second family, got %d: %s", createResp2.StatusCode, string(b))
	}

	// 3. User 3 creates Family Two
	user3 := models.User{
		Email:       "user3@example.com",
		DisplayName: "User Three",
		Status:      "active",
		AuthMethod:  "local",
	}
	_ = db.Create(&user3)
	user3Token, _, _ := sessionService.CreateSession(user3.ID)

	createReq3 := httptest.NewRequest("POST", "/api/v1/families", strings.NewReader(`{"name": "Family Three"}`))
	createReq3.Header.Set("Content-Type", "application/json")
	createReq3.AddCookie(&http.Cookie{Name: "session", Value: user3Token})
	createResp3, _ := app.Test(createReq3)
	var fam3Resp FamilyResponse
	_ = json.NewDecoder(createResp3.Body).Decode(&fam3Resp)

	// 4. User 3 invites Adult user (who is not in any family yet)
	inviteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/families/%d/invitations", fam3Resp.ID), strings.NewReader(`{"email": "adult@example.com", "role": "adult"}`))
	inviteReq.Header.Set("Content-Type", "application/json")
	inviteReq.AddCookie(&http.Cookie{Name: "session", Value: user3Token})
	inviteResp, _ := app.Test(inviteReq)
	var invResp InvitationResponse
	_ = json.NewDecoder(inviteResp.Body).Decode(&invResp)

	// 5. Parent also invites Adult to Family One
	inviteReqP := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/families/%d/invitations", famResp.ID), strings.NewReader(`{"email": "adult@example.com", "role": "adult"}`))
	inviteReqP.Header.Set("Content-Type", "application/json")
	inviteReqP.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	inviteRespP, _ := app.Test(inviteReqP)
	var invRespP InvitationResponse
	_ = json.NewDecoder(inviteRespP.Body).Decode(&invRespP)

	// Adult accepts Family One invite -> 200
	acceptReq1 := httptest.NewRequest("POST", "/api/v1/invitations/"+invRespP.InviteToken+"/accept", nil)
	acceptReq1.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	acceptResp1, _ := app.Test(acceptReq1)
	if acceptResp1.StatusCode != 200 {
		t.Fatalf("expected 200 accepting first invite, got %d", acceptResp1.StatusCode)
	}

	// Adult attempts to accept Family Three invite -> 409
	acceptReq2 := httptest.NewRequest("POST", "/api/v1/invitations/"+invResp.InviteToken+"/accept", nil)
	acceptReq2.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	acceptResp2, _ := app.Test(acceptReq2)
	if acceptResp2.StatusCode != 409 {
		b, _ := io.ReadAll(acceptResp2.Body)
		t.Fatalf("expected 409 accepting invite to second family, got %d: %s", acceptResp2.StatusCode, string(b))
	}
}
func TestUpdateAndDeleteFamily(t *testing.T) {
	app, db, sessionService, parent, adult, parentCookie, adultCookie := setupFamilyTestApp(t)
	_ = db
	_ = sessionService
	_ = parent
	_ = adult

	// 1. Parent creates family
	createReq := httptest.NewRequest("POST", "/api/v1/families", strings.NewReader(`{"name": "Initial Name"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	var famResp FamilyResponse
	_ = json.NewDecoder(createResp.Body).Decode(&famResp)
	familyID := famResp.ID

	// 2. Non-admin (adult not even member yet) attempts to rename -> 403
	renameReqBad := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/families/%d", familyID), strings.NewReader(`{"name": "Hacked Name"}`))
	renameReqBad.Header.Set("Content-Type", "application/json")
	renameReqBad.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	renameRespBad, _ := app.Test(renameReqBad)
	if renameRespBad.StatusCode != 403 {
		t.Fatalf("expected 403 for non-member/non-admin rename, got %d", renameRespBad.StatusCode)
	}

	// 3. Admin renames family with empty name -> 400
	emptyReq := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/families/%d", familyID), strings.NewReader(`{"name": "   "}`))
	emptyReq.Header.Set("Content-Type", "application/json")
	emptyReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	emptyResp, _ := app.Test(emptyReq)
	if emptyResp.StatusCode != 400 {
		t.Fatalf("expected 400 for empty family name, got %d", emptyResp.StatusCode)
	}

	// 4. Admin renames family -> 200
	renameReq := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/families/%d", familyID), strings.NewReader(`{"name": "Updated Name"}`))
	renameReq.Header.Set("Content-Type", "application/json")
	renameReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	renameResp, _ := app.Test(renameReq)
	if renameResp.StatusCode != 200 {
		t.Fatalf("expected 200 for valid rename, got %d", renameResp.StatusCode)
	}

	// Verify name changed via GET
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/families/%d", familyID), nil)
	getReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	getResp, _ := app.Test(getReq)
	var updatedFam FamilyResponse
	_ = json.NewDecoder(getResp.Body).Decode(&updatedFam)
	if updatedFam.Name != "Updated Name" {
		t.Fatalf("expected family name 'Updated Name', got '%s'", updatedFam.Name)
	}

	// 5. Non-admin attempts to delete -> 403
	delReqBad := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/families/%d", familyID), nil)
	delReqBad.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	delRespBad, _ := app.Test(delReqBad)
	if delRespBad.StatusCode != 403 {
		t.Fatalf("expected 403 for non-admin delete, got %d", delRespBad.StatusCode)
	}

	// 6. Admin deletes family -> 200
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/families/%d", familyID), nil)
	delReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	delResp, _ := app.Test(delReq)
	if delResp.StatusCode != 200 {
		t.Fatalf("expected 200 for admin delete, got %d", delResp.StatusCode)
	}

	// 7. Verify family can no longer be retrieved (403 or 404 because user is no longer active member)
	getReqAfter := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/families/%d", familyID), nil)
	getReqAfter.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	getRespAfter, _ := app.Test(getReqAfter)
	if getRespAfter.StatusCode != 403 && getRespAfter.StatusCode != 404 {
		t.Fatalf("expected 403/404 after deletion, got %d", getRespAfter.StatusCode)
	}

	// 8. Former parent can now create a new family since active membership is gone
	newFamReq := httptest.NewRequest("POST", "/api/v1/families", strings.NewReader(`{"name": "Brand New Family"}`))
	newFamReq.Header.Set("Content-Type", "application/json")
	newFamReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	newFamResp, _ := app.Test(newFamReq)
	if newFamResp.StatusCode != 201 {
		t.Fatalf("expected 201 when creating new family after deletion, got %d", newFamResp.StatusCode)
	}
}
func TestFamilySettingsAndReportAccess(t *testing.T) {
	app, db, sessionService, parent, adult, parentCookie, adultCookie := setupFamilyTestApp(t)
	_ = db
	_ = sessionService
	_ = parent
	_ = adult

	// 1. Parent creates family -> FamilySettings should exist
	createReq := httptest.NewRequest("POST", "/api/v1/families", strings.NewReader(`{"name": "Settings Family"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	var famResp FamilyResponse
	_ = json.NewDecoder(createResp.Body).Decode(&famResp)
	familyID := famResp.ID

	// 2. Verify settings via GET
	getSetReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/families/%d/settings", familyID), nil)
	getSetReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	getSetResp, _ := app.Test(getSetReq)
	if getSetResp.StatusCode != 200 {
		t.Fatalf("expected 200 on get settings, got %d", getSetResp.StatusCode)
	}
	var settings models.FamilySettings
	_ = json.NewDecoder(getSetResp.Body).Decode(&settings)
	if settings.MembersCanViewOthers != false {
		t.Fatalf("expected members_can_view_others to default to false, got %v", settings.MembersCanViewOthers)
	}

	// 3. Parent invites Adult member
	inviteReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/families/%d/invitations", familyID), strings.NewReader(`{"email": "adult@example.com", "role": "adult"}`))
	inviteReq.Header.Set("Content-Type", "application/json")
	inviteReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	inviteResp, _ := app.Test(inviteReq)
	var invResp InvitationResponse
	_ = json.NewDecoder(inviteResp.Body).Decode(&invResp)

	// Adult accepts invitation
	acceptReq := httptest.NewRequest("POST", "/api/v1/invitations/"+invResp.InviteToken+"/accept", nil)
	acceptReq.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	acceptResp, _ := app.Test(acceptReq)
	if acceptResp.StatusCode != 200 {
		t.Fatalf("expected 200 on accept invite, got %d", acceptResp.StatusCode)
	}

	// Adult can read settings
	getSetAdultReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/families/%d/settings", familyID), nil)
	getSetAdultReq.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	getSetAdultResp, _ := app.Test(getSetAdultReq)
	if getSetAdultResp.StatusCode != 200 {
		t.Fatalf("expected 200 for adult reading settings, got %d", getSetAdultResp.StatusCode)
	}

	// Non-admin (adult) cannot patch settings -> 403
	patchBad := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/families/%d/settings", familyID), strings.NewReader(`{"members_can_view_others": true}`))
	patchBad.Header.Set("Content-Type", "application/json")
	patchBad.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	patchBadResp, _ := app.Test(patchBad)
	if patchBadResp.StatusCode != 403 {
		t.Fatalf("expected 403 for adult patching settings, got %d", patchBadResp.StatusCode)
	}

	// 4. Test report access enforcement:
	// Parent sets sharing to detailed
	shareReq := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/families/%d/members/%d/sharing", familyID, parent.ID), strings.NewReader(`{"sharing": "detailed"}`))
	shareReq.Header.Set("Content-Type", "application/json")
	shareReq.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	_, _ = app.Test(shareReq)

	// Adult tries to view Parent's reports while members_can_view_others = false -> 403
	viewReq1 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/families/%d/members/%d/reports/screen-time/daily", familyID, parent.ID), nil)
	viewReq1.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	viewResp1, _ := app.Test(viewReq1)
	if viewResp1.StatusCode != 403 {
		t.Fatalf("expected 403 when members_can_view_others=false, got %d", viewResp1.StatusCode)
	}

	// 5. Admin updates settings to members_can_view_others = true -> 200
	patchGood := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/families/%d/settings", familyID), strings.NewReader(`{"members_can_view_others": true}`))
	patchGood.Header.Set("Content-Type", "application/json")
	patchGood.AddCookie(&http.Cookie{Name: "session", Value: parentCookie})
	patchGoodResp, _ := app.Test(patchGood)
	if patchGoodResp.StatusCode != 200 {
		t.Fatalf("expected 200 for admin patching settings, got %d", patchGoodResp.StatusCode)
	}

	// Adult tries to view Parent's reports now that members_can_view_others = true -> 200
	viewReq2 := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/families/%d/members/%d/reports/screen-time/daily", familyID, parent.ID), nil)
	viewReq2.AddCookie(&http.Cookie{Name: "session", Value: adultCookie})
	viewResp2, _ := app.Test(viewReq2)
	if viewResp2.StatusCode != 200 {
		t.Fatalf("expected 200 when members_can_view_others=true and sharing=detailed, got %d", viewResp2.StatusCode)
	}
}
