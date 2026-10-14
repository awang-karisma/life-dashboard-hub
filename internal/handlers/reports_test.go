package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/awang-karisma/life-dashboard-hub/internal/auth"
	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/middleware"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
	"github.com/awang-karisma/life-dashboard-hub/internal/resolver"
)

func setupReportsTestApp(t *testing.T) (*fiber.App, *database.DB, *models.User, *models.Device, *models.Device, string) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	sessionService := auth.NewSessionService(db)
	user := models.User{
		Email:       "reportuser@example.com",
		DisplayName: "Report User",
		Status:      "active",
		AuthMethod:  "local",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	d1 := models.Device{
		UserID:      user.ID,
		Label:       "phone",
		DisplayName: "Pixel Phone",
		DeviceModel: "Google Pixel 8",
		TokenHash:   "hash-phone",
		Status:      "active",
	}
	d2 := models.Device{
		UserID:      user.ID,
		Label:       "tablet",
		DisplayName: "Pixel Tablet",
		DeviceModel: "Google Pixel Tablet",
		TokenHash:   "hash-tablet",
		Status:      "active",
	}
	if err := db.Create(&d1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&d2).Error; err != nil {
		t.Fatal(err)
	}

	rawToken, _, err := sessionService.CreateSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	api := app.Group("/api/v1")
	protected := api.Group("", middleware.RequireAuth(sessionService))

	reportsHandler := NewReportsHandler(db, nil)
	reportsGroup := protected.Group("/reports")
	reportsGroup.Get("/screen-time/daily", reportsHandler.GetScreenTimeDaily)
	reportsGroup.Get("/screen-time/apps", reportsHandler.GetScreenTimeApps)
	reportsGroup.Post("/screen-time/aliases", reportsHandler.SetAppAlias)
	reportsGroup.Get("/health/daily", reportsHandler.GetHealthDailyTotals)
	reportsGroup.Get("/health/trends", reportsHandler.GetHealthTrends)
	reportsGroup.Get("/health/sleep", reportsHandler.GetSleep)
	reportsGroup.Get("/sync-status", reportsHandler.GetSyncStatus)

	dataGroup := protected.Group("/data")
	dataGroup.Get("/export", reportsHandler.ExportData)
	dataGroup.Delete("", reportsHandler.DeleteData)

	return app, db, &user, &d1, &d2, rawToken
}

func TestScreenTimeReportMultiDevice(t *testing.T) {
	app, db, _, d1, d2, sessionCookie := setupReportsTestApp(t)

	// Seed 7 days of screen time data for both devices
	baseDate, _ := time.Parse("2006-01-02", "2026-10-01")
	for i := range 7 {
		curDate := baseDate.AddDate(0, 0, i).Format("2006-01-02")

		// Device 1: 100 + i*10 minutes
		d1Mins := 100 + i*10
		d1Data, _ := json.Marshal(map[string]interface{}{
			"total_screen_time_minutes": d1Mins,
			"apps": []map[string]interface{}{
				{"package_name": "com.browser", "app_name": "Browser", "minutes": d1Mins / 2},
				{"package_name": "com.youtube", "app_name": "YouTube", "minutes": d1Mins / 2},
			},
		})
		seq := int64(1)
		rec1 := models.Record{
			DeviceID:    d1.ID,
			Type:        "screen_time",
			UUID:        fmt.Sprintf("screen_time:%d:%s", d1.ID, curDate),
			Day:         &curDate,
			Data:        string(d1Data),
			DeviceModel: d1.DeviceModel,
			Seq:         &seq,
		}
		_ = db.Create(&rec1)

		// Device 2: 50 + i*5 minutes
		d2Mins := 50 + i*5
		d2Data, _ := json.Marshal(map[string]interface{}{
			"total_screen_time_minutes": d2Mins,
			"apps": []map[string]interface{}{
				{"package_name": "com.game", "app_name": "Game", "minutes": d2Mins},
			},
		})
		rec2 := models.Record{
			DeviceID:    d2.ID,
			Type:        "screen_time",
			UUID:        fmt.Sprintf("screen_time:%d:%s", d2.ID, curDate),
			Day:         &curDate,
			Data:        string(d2Data),
			DeviceModel: d2.DeviceModel,
			Seq:         &seq,
		}
		_ = db.Create(&rec2)
	}

	// Query screen-time daily
	req := httptest.NewRequest("GET", "/api/v1/reports/screen-time/daily?from=2026-10-01&to=2026-10-07", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var dailyResp ScreenTimeDailyResponse
	_ = json.NewDecoder(resp.Body).Decode(&dailyResp)

	if len(dailyResp.Days) != 7 {
		t.Fatalf("expected 7 days, got %d", len(dailyResp.Days))
	}

	// Verify day 0 cross-device sum: d1(100) + d2(50) = 150
	day0 := dailyResp.Days[0]
	if day0.Date != "2026-10-01" {
		t.Errorf("expected 2026-10-01, got %s", day0.Date)
	}
	if day0.TotalScreenTimeMinutes != 150 {
		t.Errorf("expected cross-device sum 150, got %d", day0.TotalScreenTimeMinutes)
	}
	if day0.DeviceCount != 2 {
		t.Errorf("expected 2 devices, got %d", day0.DeviceCount)
	}

	// Verify per-app breakdown
	reqApps := httptest.NewRequest("GET", "/api/v1/reports/screen-time/apps?from=2026-10-01&to=2026-10-07", nil)
	reqApps.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respApps, _ := app.Test(reqApps)
	if respApps.StatusCode != 200 {
		t.Fatalf("expected 200 for apps, got %d", respApps.StatusCode)
	}
	var appsResp PaginatedAppsResponse
	_ = json.NewDecoder(respApps.Body).Decode(&appsResp)

	if appsResp.Total != 3 {
		t.Fatalf("expected total 3 apps, got %d", appsResp.Total)
	}
	if len(appsResp.Items) != 3 {
		t.Fatalf("expected 3 apps items, got %d", len(appsResp.Items))
	}

	// Test pagination with limit=1, offset=1
	reqPage := httptest.NewRequest("GET", "/api/v1/reports/screen-time/apps?from=2026-10-01&to=2026-10-07&limit=1&offset=1", nil)
	reqPage.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respPage, _ := app.Test(reqPage)
	var pageResp PaginatedAppsResponse
	_ = json.NewDecoder(respPage.Body).Decode(&pageResp)
	if pageResp.Total != 3 || len(pageResp.Items) != 1 || pageResp.Offset != 1 || pageResp.Limit != 1 {
		t.Fatalf("expected paginated total 3, items 1, offset 1, limit 1; got %+v", pageResp)
	}
	if pageResp.Items[0].PackageName != appsResp.Items[1].PackageName {
		t.Fatalf("expected offset 1 item to match second item of full list")
	}
}

func TestHealthReportsAndExport(t *testing.T) {
	app, db, _, d1, _, sessionCookie := setupReportsTestApp(t)

	// Seed daily_totals records
	day1 := "2026-10-10"
	day2 := "2026-10-11"
	dt1Data, _ := json.Marshal(map[string]interface{}{"steps": 6000, "distance_meters": 4500.0, "active_calories": 250.0})
	dt2Data, _ := json.Marshal(map[string]interface{}{"steps": 8000, "distance_meters": 6000.0, "active_calories": 350.0})

	seq := int64(1)
	_ = db.Create(&models.Record{
		DeviceID: d1.ID,
		Type:     "daily_totals",
		UUID:     "dt-uuid-1",
		Day:      &day1,
		Data:     string(dt1Data),
		Seq:      &seq,
		Ts:       time.Date(2026, 10, 10, 23, 59, 0, 0, time.UTC),
	})
	_ = db.Create(&models.Record{
		DeviceID: d1.ID,
		Type:     "daily_totals",
		UUID:     "dt-uuid-2",
		Day:      &day2,
		Data:     string(dt2Data),
		Seq:      &seq,
		Ts:       time.Date(2026, 10, 11, 23, 59, 0, 0, time.UTC),
	})

	// Seed sleep record
	sleepData, _ := json.Marshal(map[string]interface{}{"duration_seconds": 28800.0})
	sleepStart := time.Date(2026, 10, 10, 22, 0, 0, 0, time.UTC)
	sleepEnd := time.Date(2026, 10, 11, 6, 0, 0, 0, time.UTC)
	_ = db.Create(&models.Record{
		DeviceID: d1.ID,
		Type:     "sleep",
		UUID:     "sleep-uuid-1",
		Day:      &day2,
		Data:     string(sleepData),
		StartTs:  &sleepStart,
		Ts:       sleepEnd,
	})

	// 1. Test Health Daily Totals
	reqHealth := httptest.NewRequest("GET", "/api/v1/reports/health/daily", nil)
	reqHealth.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respHealth, _ := app.Test(reqHealth)
	var healthResp []HealthDailySummary
	_ = json.NewDecoder(respHealth.Body).Decode(&healthResp)
	if len(healthResp) != 2 {
		t.Fatalf("expected 2 health days, got %d", len(healthResp))
	}
	if healthResp[0].Steps != 6000 || healthResp[1].Steps != 8000 {
		t.Errorf("unexpected steps: %+v", healthResp)
	}

	// 2. Test Sleep
	reqSleep := httptest.NewRequest("GET", "/api/v1/reports/health/sleep", nil)
	reqSleep.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respSleep, _ := app.Test(reqSleep)
	var sleepResp []SleepSessionSummary
	_ = json.NewDecoder(respSleep.Body).Decode(&sleepResp)
	if len(sleepResp) != 1 {
		t.Fatalf("expected 1 sleep record, got %d", len(sleepResp))
	}
	if sleepResp[0].DurationMinutes != 480.0 { // 28800 / 60
		t.Errorf("expected 480 minutes, got %f", sleepResp[0].DurationMinutes)
	}

	// 3. Test Export CSV
	reqCSV := httptest.NewRequest("GET", "/api/v1/data/export?format=csv", nil)
	reqCSV.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respCSV, _ := app.Test(reqCSV)
	if respCSV.StatusCode != 200 {
		t.Fatalf("expected 200 for csv export, got %d", respCSV.StatusCode)
	}
	csvBytes, _ := io.ReadAll(respCSV.Body)
	csvStr := string(csvBytes)
	if !strings.Contains(csvStr, "dt-uuid-1") || !strings.Contains(csvStr, "sleep-uuid-1") {
		t.Errorf("csv missing expected records: %s", csvStr)
	}

	// 4. Test Data Deletion
	reqDel := httptest.NewRequest("DELETE", "/api/v1/data?from=2026-10-10&to=2026-10-10", nil)
	reqDel.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respDel, _ := app.Test(reqDel)
	if respDel.StatusCode != 200 {
		t.Fatalf("expected 200 on data delete, got %d", respDel.StatusCode)
	}

	// Verify day 1 is deleted and day 2 remains
	var remaining []models.Record
	_ = db.Find(&remaining)
	for _, r := range remaining {
		if r.Day != nil && *r.Day == "2026-10-10" {
			t.Errorf("record for 2026-10-10 was not deleted")
		}
	}
}
func TestScreenTimeAppsCategories(t *testing.T) {
	app, db, _, d1, _, sessionCookie := setupReportsTestApp(t)

	// Setup resolver with DB
	res := resolver.NewPackageResolver(db)

	// Insert app metadata directly into DB
	meta1 := models.AppMetadata{
		PackageName: "com.spotify.music",
		AppName:     "Spotify",
		Category:    "MUSIC_AND_AUDIO",
		Source:      "builtin",
		ResolvedAt:  time.Now().UTC(),
	}
	_ = db.Save(&meta1)

	// Seed screen time record for d1
	curDate := "2026-10-12"
	dData, _ := json.Marshal(map[string]interface{}{
		"total_screen_time_minutes": 120,
		"apps": []map[string]interface{}{
			{"package_name": "com.spotify.music", "app_name": "Spotify", "minutes": 70},
			{"package_name": "com.unknown.app", "app_name": "", "minutes": 50},
		},
	})
	rec := models.Record{
		DeviceID:    d1.ID,
		Type:        "screen_time",
		UUID:        fmt.Sprintf("screen_time:%d:%s", d1.ID, curDate),
		Day:         &curDate,
		Data:        string(dData),
		DeviceModel: d1.DeviceModel,
	}
	_ = db.Create(&rec)

	// Also attach resolver to handler
	reportsHandler := NewReportsHandler(db, res)
	app.Get("/api/v1/reports/screen-time/apps-with-resolver", reportsHandler.GetScreenTimeApps)

	req := httptest.NewRequest("GET", "/api/v1/reports/screen-time/apps-with-resolver?from=2026-10-01&to=2026-10-13", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var paginatedResp PaginatedAppsResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginatedResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	apps := paginatedResp.Items

	if paginatedResp.Total != 2 {
		t.Fatalf("expected total 2 apps, got %d", paginatedResp.Total)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	// Find spotify and unknown
	var spotify, unknown *AppUsageSummary
	for i := range apps {
		if apps[i].PackageName == "com.spotify.music" {
			spotify = &apps[i]
		} else if apps[i].PackageName == "com.unknown.app" {
			unknown = &apps[i]
		}
	}

	if spotify == nil {
		t.Fatal("com.spotify.music not found in response")
	}
	if spotify.Category != "MUSIC_AND_AUDIO" {
		t.Errorf("expected category 'MUSIC_AND_AUDIO', got '%s'", spotify.Category)
	}
	if spotify.AppName != "Spotify" {
		t.Errorf("expected app_name 'Spotify', got '%s'", spotify.AppName)
	}

	if unknown == nil {
		t.Fatal("com.unknown.app not found in response")
	}
	if unknown.Category != "" {
		t.Errorf("expected empty category for unresolved app, got '%s'", unknown.Category)
	}
	if unknown.AppName != "App" {
		t.Errorf("expected fallback app_name 'App', got '%s'", unknown.AppName)
	}
}

func TestSetAppAliasUserScoped(t *testing.T) {
	app, db, user1, d1, _, sessionCookie1 := setupReportsTestApp(t)
	sessionService := auth.NewSessionService(db)

	// Create user 2 with device
	user2 := models.User{
		Email:       "user2@example.com",
		DisplayName: "User Two",
		Status:      "active",
		AuthMethod:  "local",
	}
	if err := db.Create(&user2).Error; err != nil {
		t.Fatal(err)
	}
	dUser2 := models.Device{
		UserID:      user2.ID,
		Label:       "phone2",
		DisplayName: "User 2 Phone",
		TokenHash:   "hash-user2",
		Status:      "active",
	}
	if err := db.Create(&dUser2).Error; err != nil {
		t.Fatal(err)
	}
	sessionCookie2, _, err := sessionService.CreateSession(user2.ID)
	if err != nil {
		t.Fatal(err)
	}

	day := "2026-10-12"
	seq := int64(1)
	appData, _ := json.Marshal(map[string]interface{}{
		"total_screen_time_minutes": 60,
		"apps": []map[string]interface{}{
			{"package_name": "com.custom.notes", "app_name": "Notes", "minutes": 60},
		},
	})

	// Seed record for user1
	_ = db.Create(&models.Record{
		DeviceID: d1.ID,
		Type:     "screen_time",
		UUID:     "st-u1",
		Day:      &day,
		Data:     string(appData),
		Seq:      &seq,
	})
	// Seed record for user2
	_ = db.Create(&models.Record{
		DeviceID: dUser2.ID,
		Type:     "screen_time",
		UUID:     "st-u2",
		Day:      &day,
		Data:     string(appData),
		Seq:      &seq,
	})

	// User 1 sets custom alias
	aliasBody := `{"package_name": "com.custom.notes", "app_name": "My Secret Diary"}`
	reqAlias := httptest.NewRequest("POST", "/api/v1/reports/screen-time/aliases", strings.NewReader(aliasBody))
	reqAlias.Header.Set("Content-Type", "application/json")
	reqAlias.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie1})
	respAlias, err := app.Test(reqAlias)
	if err != nil {
		t.Fatal(err)
	}
	if respAlias.StatusCode != 200 {
		t.Fatalf("expected 200 for SetAppAlias, got %d", respAlias.StatusCode)
	}

	// Verify in app_aliases table
	var aliasEntry models.AppAlias
	if err := db.Where("user_id = ? AND package_name = ?", user1.ID, "com.custom.notes").First(&aliasEntry).Error; err != nil {
		t.Fatalf("alias entry not found for user 1: %v", err)
	}
	if aliasEntry.AppName != "My Secret Diary" {
		t.Errorf("expected 'My Secret Diary', got '%s'", aliasEntry.AppName)
	}

	// Query user 1 screen-time apps -> custom alias applied
	reqApps1 := httptest.NewRequest("GET", "/api/v1/reports/screen-time/apps?from=2026-10-12&to=2026-10-12", nil)
	reqApps1.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie1})
	respApps1, _ := app.Test(reqApps1)
	var appsResp1 PaginatedAppsResponse
	_ = json.NewDecoder(respApps1.Body).Decode(&appsResp1)
	if len(appsResp1.Items) != 1 || appsResp1.Items[0].AppName != "My Secret Diary" {
		t.Errorf("expected user 1 to see 'My Secret Diary', got %+v", appsResp1.Items)
	}

	// Query user 2 screen-time apps -> User 1's custom alias NOT applied
	reqApps2 := httptest.NewRequest("GET", "/api/v1/reports/screen-time/apps?from=2026-10-12&to=2026-10-12", nil)
	reqApps2.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie2})
	respApps2, _ := app.Test(reqApps2)
	var appsResp2 PaginatedAppsResponse
	_ = json.NewDecoder(respApps2.Body).Decode(&appsResp2)
	if len(appsResp2.Items) != 1 || appsResp2.Items[0].AppName == "My Secret Diary" {
		t.Errorf("user 2 leaked user 1 alias: %+v", appsResp2.Items)
	}
}

func TestDeleteDataFullCascade(t *testing.T) {
	app, db, _, d1, _, sessionCookie := setupReportsTestApp(t)

	// Seed Record, Bucket, Tombstone, Payload for d1
	day := "2026-10-15"
	rec := models.Record{
		DeviceID: d1.ID,
		Type:     "steps",
		UUID:     "rec-del-1",
		Day:      &day,
		Data:     `{}`,
	}
	_ = db.Create(&rec)

	bucket := models.Bucket{
		DeviceID:      d1.ID,
		Type:          "heart_rate",
		PayloadSource: "health_connect",
		BucketStart:   time.Now().UTC(),
		BucketEnd:     time.Now().UTC().Add(time.Hour),
	}
	_ = db.Create(&bucket)

	tombstone := models.Tombstone{
		DeviceID: d1.ID,
		Type:     "steps",
		UUID:     "tomb-del-1",
	}
	_ = db.Create(&tombstone)

	payload := models.Payload{
		DeviceID:   d1.ID,
		Source:     "companion",
		BodySHA256: "sha-del-1",
		Body:       `{}`,
	}
	_ = db.Create(&payload)

	// Call DELETE /api/v1/data (no from/to -> full scope for user)
	reqDel := httptest.NewRequest("DELETE", "/api/v1/data", nil)
	reqDel.AddCookie(&http.Cookie{Name: "session", Value: sessionCookie})
	respDel, err := app.Test(reqDel)
	if err != nil {
		t.Fatal(err)
	}
	if respDel.StatusCode != 200 {
		t.Fatalf("expected 200 for full data delete, got %d", respDel.StatusCode)
	}

	var recCount, bucketCount, tombCount, payloadCount int64
	db.Model(&models.Record{}).Where("device_id = ?", d1.ID).Count(&recCount)
	db.Model(&models.Bucket{}).Where("device_id = ?", d1.ID).Count(&bucketCount)
	db.Model(&models.Tombstone{}).Where("device_id = ?", d1.ID).Count(&tombCount)
	db.Model(&models.Payload{}).Where("device_id = ?", d1.ID).Count(&payloadCount)

	if recCount != 0 {
		t.Errorf("expected 0 records, got %d", recCount)
	}
	if bucketCount != 0 {
		t.Errorf("expected 0 buckets, got %d", bucketCount)
	}
	if tombCount != 0 {
		t.Errorf("expected 0 tombstones, got %d", tombCount)
	}
	if payloadCount != 0 {
		t.Errorf("expected 0 payloads, got %d", payloadCount)
	}
}
