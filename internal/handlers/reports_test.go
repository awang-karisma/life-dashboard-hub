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

	reportsHandler := NewReportsHandler(db)
	reportsGroup := protected.Group("/reports")
	reportsGroup.Get("/screen-time/daily", reportsHandler.GetScreenTimeDaily)
	reportsGroup.Get("/screen-time/apps", reportsHandler.GetScreenTimeApps)
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
	var appsResp []AppUsageSummary
	_ = json.NewDecoder(respApps.Body).Decode(&appsResp)

	if len(appsResp) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(appsResp))
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
