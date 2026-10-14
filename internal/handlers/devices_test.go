package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/awang-karisma/life-dashboard-hub/internal/auth"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/middleware"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
)

func setupDeviceTestApp(t *testing.T) (*fiber.App, *database.DB, *auth.SessionService, *models.User, string) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	sessionService := auth.NewSessionService(db)
	user := models.User{
		Email:       "deviceuser@example.com",
		DisplayName: "Device User",
		Status:      "active",
		AuthMethod:  "local",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	rawToken, _, err := sessionService.CreateSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	api := app.Group("/api/v1")
	protected := api.Group("", middleware.RequireAuth(sessionService))

	devicesHandler := NewDevicesHandler(db, "https://dashboard.example.com")
	devicesGroup := protected.Group("/devices")
	devicesGroup.Post("", devicesHandler.CreateDevice)
	devicesGroup.Get("", devicesHandler.ListDevices)
	devicesGroup.Get("/:id", devicesHandler.GetDevice)
	devicesGroup.Patch("/:id", devicesHandler.UpdateDevice)
	devicesGroup.Delete("/:id", devicesHandler.DeleteDevice)
	devicesGroup.Post("/:id/revoke", devicesHandler.RevokeDevice)
	devicesGroup.Post("/:id/regenerate", devicesHandler.RegenerateDeviceCredentials)

	return app, db, sessionService, &user, rawToken
}

func TestDeviceCRUDAndCredentials(t *testing.T) {
	app, db, _, user, sessionCookie := setupDeviceTestApp(t)

	// 1. Create Device
	createBody := `{"label": "pixel-8-pro", "display_name": "Pixel 8 Pro"}`
	reqCreate := httptest.NewRequest("POST", "/api/v1/devices", strings.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})

	respCreate, err := app.Test(reqCreate)
	if err != nil {
		t.Fatal(err)
	}
	defer respCreate.Body.Close()

	if respCreate.StatusCode != 201 {
		b, _ := io.ReadAll(respCreate.Body)
		t.Fatalf("expected 201, got %d: %s", respCreate.StatusCode, string(b))
	}

	var credResp DeviceCredentialsResponse
	_ = json.NewDecoder(respCreate.Body).Decode(&credResp)

	if credResp.Label != "pixel-8-pro" || credResp.DisplayName != "Pixel 8 Pro" {
		t.Errorf("unexpected device created: %+v", credResp)
	}
	if credResp.WebhookURL == "" || !strings.Contains(credResp.WebhookURL, "/webhooks/v1/devices/") {
		t.Errorf("unexpected webhook url: %s", credResp.WebhookURL)
	}
	if credResp.WebhookSecret == "" || len(credResp.WebhookSecret) != 64 {
		t.Errorf("unexpected webhook secret length: %s", credResp.WebhookSecret)
	}

	// Verify Pairing QR format:
	// lifedashboard://pair#v=1&url=<URL>&secret=<SECRET>&name=<NAME>&sources=health_connect,screen_time
	qr := credResp.PairingQR
	if !strings.HasPrefix(qr, "lifedashboard://pair#v=1&") {
		t.Errorf("unexpected pairing qr prefix: %s", qr)
	}
	if !strings.Contains(qr, "url="+url.QueryEscape(credResp.WebhookURL)) {
		t.Errorf("pairing qr does not contain escaped url: %s", qr)
	}
	if !strings.Contains(qr, "secret="+credResp.WebhookSecret) {
		t.Errorf("pairing qr does not contain secret: %s", qr)
	}
	if !strings.Contains(qr, "sources=health_connect,screen_time") {
		t.Errorf("pairing qr does not contain sources: %s", qr)
	}
	if !strings.Contains(qr, "name=Life%20Dashboard%20Hub") {
		t.Errorf("pairing qr does not contain instance name: %s", qr)
	}

	deviceID := credResp.ID

	// 2. Conflict on duplicate label
	reqDup := httptest.NewRequest("POST", "/api/v1/devices", strings.NewReader(createBody))
	reqDup.Header.Set("Content-Type", "application/json")
	reqDup.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respDup, _ := app.Test(reqDup)
	if respDup.StatusCode != 409 {
		t.Errorf("expected 409 on duplicate label, got %d", respDup.StatusCode)
	}

	// 3. List Devices (verify webhook_secret and token are NOT returned)
	reqList := httptest.NewRequest("GET", "/api/v1/devices", nil)
	reqList.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respList, _ := app.Test(reqList)
	if respList.StatusCode != 200 {
		t.Fatalf("expected 200 on list devices, got %d", respList.StatusCode)
	}
	var devList []DeviceResponse
	_ = json.NewDecoder(respList.Body).Decode(&devList)
	if len(devList) != 1 || devList[0].ID != deviceID {
		t.Fatalf("unexpected devices list: %+v", devList)
	}

	// 4. Get Device by ID
	reqGet := httptest.NewRequest("GET", "/api/v1/devices/1", nil)
	reqGet.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respGet, _ := app.Test(reqGet)
	if respGet.StatusCode != 200 {
		t.Fatalf("expected 200 on get device, got %d", respGet.StatusCode)
	}

	// 5. Update Device
	reqUpdate := httptest.NewRequest("PATCH", "/api/v1/devices/1", strings.NewReader(`{"display_name": "Pixel 8 Pro (Work)"}`))
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respUpdate, _ := app.Test(reqUpdate)
	if respUpdate.StatusCode != 200 {
		t.Fatalf("expected 200 on update device, got %d", respUpdate.StatusCode)
	}
	var updatedDev DeviceResponse
	_ = json.NewDecoder(respUpdate.Body).Decode(&updatedDev)
	if updatedDev.DisplayName != "Pixel 8 Pro (Work)" {
		t.Errorf("expected updated display_name, got %s", updatedDev.DisplayName)
	}

	// 6. Regenerate Credentials
	reqRegen := httptest.NewRequest("POST", "/api/v1/devices/1/regenerate", nil)
	reqRegen.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respRegen, _ := app.Test(reqRegen)
	if respRegen.StatusCode != 200 {
		t.Fatalf("expected 200 on regenerate, got %d", respRegen.StatusCode)
	}
	var regenCreds DeviceCredentialsResponse
	_ = json.NewDecoder(respRegen.Body).Decode(&regenCreds)
	if regenCreds.WebhookURL == credResp.WebhookURL {
		t.Errorf("expected new webhook url on regenerate, got same")
	}
	if regenCreds.WebhookSecret == credResp.WebhookSecret {
		t.Errorf("expected new webhook secret on regenerate, got same")
	}
	if !strings.Contains(regenCreds.PairingQR, "name=Life%20Dashboard%20Hub") {
		t.Errorf("regenerated pairing qr does not contain instance name: %s", regenCreds.PairingQR)
	}

	// 7. Revoke Device
	reqRevoke := httptest.NewRequest("POST", "/api/v1/devices/1/revoke", nil)
	reqRevoke.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respRevoke, _ := app.Test(reqRevoke)
	if respRevoke.StatusCode != 200 {
		t.Fatalf("expected 200 on revoke, got %d", respRevoke.StatusCode)
	}
	var revokedDev DeviceResponse
	_ = json.NewDecoder(respRevoke.Body).Decode(&revokedDev)
	if revokedDev.Status != "revoked" || revokedDev.RevokedAt == nil {
		t.Errorf("expected status revoked, got %+v", revokedDev)
	}

	// 8. Delete Device
	// Insert dummy record to test cascading delete
	rec := models.Record{
		DeviceID: deviceID,
		Type:     "screen_time",
		UUID:     "dummy-uuid-1",
		Data:     "{}",
	}
	_ = db.Create(&rec)

	reqDel := httptest.NewRequest("DELETE", "/api/v1/devices/1", nil)
	reqDel.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respDel, _ := app.Test(reqDel)
	if respDel.StatusCode != 200 {
		t.Fatalf("expected 200 on delete, got %d", respDel.StatusCode)
	}

	// Verify device and records are gone
	var checkDev models.Device
	if err := db.Where("id = ?", deviceID).First(&checkDev).Error; err == nil {
		t.Error("expected device to be deleted")
	}
	var checkRec models.Record
	if err := db.Where("device_id = ?", deviceID).First(&checkRec).Error; err == nil {
		t.Error("expected associated records to be deleted")
	}

	_ = user
}
