package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
)

func setupTestApp(t *testing.T) (*fiber.App, *database.DB, *models.Device, string, string) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	rawToken := "test-token-secret-xyz-12345"
	tokenHash := HashToken(rawToken)
	secret := "test-hmac-secret-67890"

	user := models.User{
		Email: "test@example.com",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	device := models.Device{
		UserID:        user.ID,
		Label:         "pixel-8",
		DisplayName:   "My Pixel 8",
		TokenHash:     tokenHash,
		WebhookSecret: secret,
		Status:        "active",
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	app := fiber.New()
	wh := NewWebhookHandler(db, 32*1024*1024, "UTC", NewIngestionBus())
	app.Post("/webhooks/v1/devices/:deviceToken", wh.HandleWebhook)

	return app, db, &device, rawToken, secret
}

func signBody(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookScreenTimeIngestion(t *testing.T) {
	app, db, device, rawToken, secret := setupTestApp(t)

	payload := map[string]interface{}{
		"source":      "screen_time",
		"sequence":    1,
		"app_version": "1.23.0",
		"built_at":    "2026-10-11T12:00:00Z",
		"device":      "Google Pixel 8",
		"days": []map[string]interface{}{
			{
				"date":                      "2026-10-11",
				"total_screen_time_minutes": 185,
				"apps": []map[string]interface{}{
					{"package_name": "com.browser", "app_name": "Browser", "minutes": 120},
					{"package_name": "com.chat", "app_name": "Chat", "minutes": 65},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	req.ContentLength = int64(len(bodyBytes))
	req.Header.Set("X-Signature", signBody(bodyBytes, secret))

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var respJSON map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&respJSON)
	if respJSON["stored"].(float64) != 1 {
		t.Errorf("expected stored=1, got %v", respJSON["stored"])
	}

	// Verify database record
	var rec models.Record
	if err := db.Where("device_id = ? AND type = 'screen_time'", device.ID).First(&rec).Error; err != nil {
		t.Fatalf("record not found in db: %v", err)
	}
	if rec.Day == nil || *rec.Day != "2026-10-11" {
		t.Errorf("expected day 2026-10-11, got %v", rec.Day)
	}

	// Verify device last_sync was updated
	var updatedDevice models.Device
	_ = db.First(&updatedDevice, device.ID)
	if updatedDevice.LastSyncAt == nil {
		t.Error("expected LastSyncAt to be set")
	}
	if updatedDevice.DeviceModel != "Google Pixel 8" {
		t.Errorf("expected DeviceModel 'Google Pixel 8', got '%s'", updatedDevice.DeviceModel)
	}
	if updatedDevice.CollectorVersion != "1.23.0" {
		t.Errorf("expected CollectorVersion '1.23.0', got '%s'", updatedDevice.CollectorVersion)
	}

	// Duplicate test: post identical payload again -> duplicate: true
	req2 := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.ContentLength = int64(len(bodyBytes))
	req2.Header.Set("X-Signature", signBody(bodyBytes, secret))

	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	var resp2JSON map[string]interface{}
	_ = json.NewDecoder(resp2.Body).Decode(&resp2JSON)
	if resp2JSON["duplicate"] != true {
		t.Errorf("expected duplicate=true, got %v", resp2JSON)
	}
}

func TestWebhookAuthAndValidationFailures(t *testing.T) {
	app, db, device, rawToken, secret := setupTestApp(t)

	bodyBytes := []byte(`{"source": "screen_time", "ping": true}`)

	// 1. Missing signature when secret is set -> 401
	req1 := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req1.ContentLength = int64(len(bodyBytes))
	resp1, _ := app.Test(req1)
	if resp1.StatusCode != 401 {
		t.Errorf("expected 401 for missing signature, got %d", resp1.StatusCode)
	}

	// 2. Invalid signature -> 401
	req2 := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req2.ContentLength = int64(len(bodyBytes))
	req2.Header.Set("X-Signature", "sha256=badbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbad")
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != 401 {
		t.Errorf("expected 401 for invalid signature, got %d", resp2.StatusCode)
	}

	// 3. Unknown device token -> 404
	req3 := httptest.NewRequest("POST", "/webhooks/v1/devices/nonexistent-token", bytes.NewReader(bodyBytes))
	req3.ContentLength = int64(len(bodyBytes))
	req3.Header.Set("X-Signature", signBody(bodyBytes, secret))
	resp3, _ := app.Test(req3)
	if resp3.StatusCode != 404 {
		t.Errorf("expected 404 for unknown token, got %d", resp3.StatusCode)
	}

	// 4. Revoked device -> 404
	device.Status = "revoked"
	now := time.Now().UTC()
	device.RevokedAt = &now
	_ = db.Save(device)

	req4 := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req4.ContentLength = int64(len(bodyBytes))
	req4.Header.Set("X-Signature", signBody(bodyBytes, secret))
	resp4, _ := app.Test(req4)
	if resp4.StatusCode != 404 {
		t.Errorf("expected 404 for revoked device, got %d", resp4.StatusCode)
	}

	// 5. Test ping on active device -> 200 with {"stored": 0, "ping": true}
	device.Status = "active"
	_ = db.Save(device)

	req5 := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req5.ContentLength = int64(len(bodyBytes))
	req5.Header.Set("X-Signature", signBody(bodyBytes, secret))
	resp5, _ := app.Test(req5)
	if resp5.StatusCode != 200 {
		t.Errorf("expected 200 for ping, got %d", resp5.StatusCode)
	}
	var pingResp map[string]interface{}
	_ = json.NewDecoder(resp5.Body).Decode(&pingResp)
	if pingResp["ping"] != true {
		t.Errorf("expected ping=true, got %v", pingResp)
	}
}

func TestWebhookHealthConnectIngestion(t *testing.T) {
	app, db, device, rawToken, secret := setupTestApp(t)

	payload := map[string]interface{}{
		"source":          "health_connect",
		"sequence":        2,
		"app_version":     "1.23.0",
		"built_at":        "2026-10-11T12:00:00Z",
		"device":          "Pixel 8",
		"window_complete": true,
		"daily_totals": []map[string]interface{}{
			{
				"date":            "2026-10-11",
				"steps":           10500,
				"distance_meters": 7800.5,
				"active_calories": 420.0,
			},
		},
		"steps": []map[string]interface{}{
			{
				"uuid":       "step-record-1",
				"time":       "2026-10-11T10:00:00Z",
				"count":      500,
				"source_app": "com.google.android.apps.fitness",
			},
		},
		"deleted_records": []interface{}{
			map[string]interface{}{
				"uuid": "old-step-record-999",
				"type": "steps",
			},
		},
		"_resolutions": map[string]interface{}{
			"heart_rate": []map[string]interface{}{
				{
					"bucket_start": "2026-10-11T10:00:00Z",
					"bucket_end":   "2026-10-11T11:00:00Z",
					"sample_count": 60,
					"avg":          72.5,
					"min":          58.0,
					"max":          95.0,
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req.ContentLength = int64(len(bodyBytes))
	req.Header.Set("X-Signature", signBody(bodyBytes, secret))

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var respJSON map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&respJSON)
	if respJSON["stored"].(float64) != 2 { // 1 daily_totals + 1 steps
		t.Errorf("expected stored=2, got %v", respJSON["stored"])
	}
	if respJSON["buckets"].(float64) != 1 {
		t.Errorf("expected buckets=1, got %v", respJSON["buckets"])
	}
	if respJSON["deleted"].(float64) != 1 {
		t.Errorf("expected deleted=1, got %v", respJSON["deleted"])
	}

	// Verify tombstone was created
	var tomb models.Tombstone
	if err := db.Where("device_id = ? AND uuid = 'old-step-record-999'", device.ID).First(&tomb).Error; err != nil {
		t.Errorf("expected tombstone in db: %v", err)
	}
}

func TestWebhookPublishesToIngestionBus(t *testing.T) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatal(err)
	}

	rawToken := "bus-test-token-12345"
	tokenHash := HashToken(rawToken)
	secret := "bus-secret-67890"

	user := models.User{Email: "bustest@example.com"}
	_ = db.Create(&user)

	device := models.Device{
		UserID:        user.ID,
		Label:         "bus-phone",
		DisplayName:   "Bus Test Phone",
		TokenHash:     tokenHash,
		WebhookSecret: secret,
		Status:        "active",
	}
	_ = db.Create(&device)

	bus := NewIngestionBus()
	subID, ch := bus.Subscribe()
	defer bus.Unsubscribe(subID)

	app := fiber.New()
	wh := NewWebhookHandler(db, 32*1024*1024, "UTC", bus)
	app.Post("/webhooks/v1/devices/:deviceToken", wh.HandleWebhook)

	payload := map[string]interface{}{
		"type": "ping",
	}
	bodyBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/webhooks/v1/devices/"+rawToken, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	req.ContentLength = int64(len(bodyBytes))
	req.Header.Set("X-Signature", signBody(bodyBytes, secret))

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	select {
	case ev := <-ch:
		if ev.Status != "PING_OK" {
			t.Errorf("expected Status PING_OK, got %s", ev.Status)
		}
		if ev.DeviceName != "Bus Test Phone" {
			t.Errorf("expected DeviceName Bus Test Phone, got %s", ev.DeviceName)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event on bus")
	}
}
