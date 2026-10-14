package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/models"
	"github.com/owen282000/life-dashboard-hub/internal/resolver"
	"gorm.io/gorm"
)

type ReportsHandler struct {
	DB       *database.DB
	Resolver *resolver.PackageResolver
}

func NewReportsHandler(db *database.DB, res *resolver.PackageResolver) *ReportsHandler {
	return &ReportsHandler{
		DB:       db,
		Resolver: res,
	}
}

// Helper to get allowed device IDs for user
func (h *ReportsHandler) getUserDeviceIDs(userID uint, reqDeviceID string) ([]uint, error) {
	var devices []models.Device
	query := h.DB.Where("user_id = ?", userID)
	if reqDeviceID != "" {
		dID, err := strconv.ParseUint(reqDeviceID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid device_id")
		}
		query = query.Where("id = ?", dID)
	}

	if err := query.Find(&devices).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, len(devices))
	for i, d := range devices {
		ids[i] = d.ID
	}
	return ids, nil
}

type ScreenTimeDailyResponse struct {
	From        string                 `json:"from"`
	To          string                 `json:"to"`
	Days        []ScreenTimeDaySummary `json:"days"`
	DeviceUsage []DeviceDaySummary     `json:"device_usage"`
}

type ScreenTimeDaySummary struct {
	Date                   string `json:"date"`
	TotalScreenTimeMinutes int64  `json:"total_screen_time_minutes"` // Total reported device usage across devices
	FilteredMinutes        int64  `json:"filtered_screen_time_minutes"`
	DeviceCount            int    `json:"device_count"`
}

type DeviceDaySummary struct {
	DeviceID               uint   `json:"device_id"`
	DeviceModel            string `json:"device_model"`
	Date                   string `json:"date"`
	TotalScreenTimeMinutes int64  `json:"total_screen_time_minutes"`
	FilteredMinutes        int64  `json:"filtered_screen_time_minutes"`
}

type AppUsageSummary struct {
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name"`
	Category    string `json:"category"`
	Minutes     int64  `json:"minutes"`
}
type PaginatedAppsResponse struct {
	Total  int               `json:"total"`
	Offset int               `json:"offset"`
	Limit  int               `json:"limit"`
	Items  []AppUsageSummary `json:"items"`
}


type HealthDailySummary struct {
	Date           string  `json:"date"`
	Steps          int64   `json:"steps"`
	DistanceMeters float64 `json:"distance_meters"`
	Calories       float64 `json:"calories"`
}

type TrendSummary struct {
	Period        string  `json:"period"`
	StepsTotal    int64   `json:"steps_total"`
	StepsAvg      float64 `json:"steps_avg"`
	DistanceTotal float64 `json:"distance_total"`
	CaloriesTotal float64 `json:"calories_total"`
	DaysCount     int     `json:"days_count"`
}

type SleepSessionSummary struct {
	RecordID        string    `json:"record_id"`
	WakeDate        string    `json:"wake_date"`
	DurationMinutes float64   `json:"duration_minutes"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
}

type SyncStatusResponse struct {
	Devices []SyncStatusItem `json:"devices"`
}

type SyncStatusItem struct {
	DeviceID         uint       `json:"device_id"`
	DisplayName      string     `json:"display_name"`
	DeviceModel      string     `json:"device_model"`
	CollectorVersion string     `json:"collector_version"`
	Status           string     `json:"status"`
	LastSyncAt       *time.Time `json:"last_sync_at"`
	LastSyncSource   string     `json:"last_sync_source"`
	LastSyncError    string     `json:"last_sync_error"`
	IsStale          bool       `json:"is_stale"`
}

// GetScreenTimeDaily godoc
// @Summary      Get daily screen time report
// @Description  Aggregates daily screen time per-device and combined total reported device usage
// @Tags         reports
// @Produce      json
// @Param        from query string false "From date YYYY-MM-DD"
// @Param        to query string false "To date YYYY-MM-DD"
// @Param        device_id query string false "Filter by device ID"
// @Success      200 {object} ScreenTimeDailyResponse
// @Failure      400 {object} ErrorResponse
// @Failure      401 {object} ErrorResponse
// @Router       /reports/screen-time/daily [get]
func (h *ReportsHandler) GetScreenTimeDaily(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	from := c.Query("from")
	to := c.Query("to")
	deviceIDParam := c.Query("device_id")

	deviceIDs, err := h.getUserDeviceIDs(userID, deviceIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: err.Error()})
	}
	if len(deviceIDs) == 0 {
		return c.JSON(ScreenTimeDailyResponse{From: from, To: to, Days: []ScreenTimeDaySummary{}, DeviceUsage: []DeviceDaySummary{}})
	}

	query := h.DB.Where("type = 'screen_time' AND device_id IN (?)", deviceIDs)
	if from != "" {
		query = query.Where("day >= ?", from)
	}
	if to != "" {
		query = query.Where("day <= ?", to)
	}

	var records []models.Record
	if err := query.Order("day ASC, seq DESC, built_at DESC").Find(&records).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to query records"})
	}

	// Deduplicate per (device_id, day)
	seen := make(map[string]bool)
	var deduped []models.Record
	for _, r := range records {
		if r.Day == nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", r.DeviceID, *r.Day)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, r)
		}
	}

	// Aggregate per day and device
	dayMap := make(map[string]*ScreenTimeDaySummary)
	var deviceUsage []DeviceDaySummary

	for _, r := range deduped {
		dayStr := *r.Day
		var dataMap map[string]interface{}
		_ = json.Unmarshal([]byte(r.Data), &dataMap)

		var totalMinutes int64
		if val, ok := dataMap["total_screen_time_minutes"].(float64); ok {
			totalMinutes = int64(val)
		}
		var filteredMinutes int64
		if val, ok := dataMap["filtered_screen_time_minutes"].(float64); ok {
			filteredMinutes = int64(val)
		}

		deviceUsage = append(deviceUsage, DeviceDaySummary{
			DeviceID:               r.DeviceID,
			DeviceModel:            r.DeviceModel,
			Date:                   dayStr,
			TotalScreenTimeMinutes: totalMinutes,
			FilteredMinutes:        filteredMinutes,
		})

		if _, exists := dayMap[dayStr]; !exists {
			dayMap[dayStr] = &ScreenTimeDaySummary{
				Date: dayStr,
			}
		}
		dayMap[dayStr].TotalScreenTimeMinutes += totalMinutes
		dayMap[dayStr].FilteredMinutes += filteredMinutes
		dayMap[dayStr].DeviceCount++
	}

	days := make([]ScreenTimeDaySummary, 0, len(dayMap))
	for _, summary := range dayMap {
		days = append(days, *summary)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date < days[j].Date
	})

	return c.JSON(ScreenTimeDailyResponse{
		From:        from,
		To:          to,
		Days:        days,
		DeviceUsage: deviceUsage,
	})
}

// GetScreenTimeApps godoc
// @Summary      Get app usage breakdown
// @Description  Aggregates foreground app minutes from screen time records
// @Tags         reports
// @Produce      json
// @Param        from query string false "From date YYYY-MM-DD"
// @Param        to query string false "To date YYYY-MM-DD"
// @Param        device_id query string false "Filter by device ID"
// @Param        limit query int false "Top N apps (default 20)"
// @Success      200 {array} AppUsageSummary
// @Router       /reports/screen-time/apps [get]
func (h *ReportsHandler) GetScreenTimeApps(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	from := c.Query("from")
	to := c.Query("to")
	deviceIDParam := c.Query("device_id")

	limit := 20
	if lim := c.Query("limit"); lim != "" {
		if l, err := strconv.Atoi(lim); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if off := c.Query("offset"); off != "" {
		if o, err := strconv.Atoi(off); err == nil && o >= 0 {
			offset = o
		}
	}

	deviceIDs, err := h.getUserDeviceIDs(userID, deviceIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: err.Error()})
	}
	if len(deviceIDs) == 0 {
		return c.JSON(PaginatedAppsResponse{
			Total:  0,
			Offset: offset,
			Limit:  limit,
			Items:  []AppUsageSummary{},
		})
	}

	query := h.DB.Where("type = 'screen_time' AND device_id IN (?)", deviceIDs)
	if from != "" {
		query = query.Where("day >= ?", from)
	}
	if to != "" {
		query = query.Where("day <= ?", to)
	}

	var records []models.Record
	if err := query.Order("day ASC, seq DESC, built_at DESC").Find(&records).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to query records"})
	}

	// Deduplicate per (device_id, day)
	seen := make(map[string]bool)
	appTotals := make(map[string]*AppUsageSummary)

	for _, r := range records {
		if r.Day == nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", r.DeviceID, *r.Day)
		if seen[key] {
			continue
		}
		seen[key] = true

		var dataMap map[string]interface{}
		if err := json.Unmarshal([]byte(r.Data), &dataMap); err != nil {
			continue
		}

		appsList, ok := dataMap["apps"].([]interface{})
		if !ok {
			continue
		}

		for _, item := range appsList {
			appMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			pkgName, _ := appMap["package_name"].(string)
			if pkgName == "" {
				pkgName, _ = appMap["package"].(string)
			}
			if pkgName == "" {
				continue
			}
			appName, _ := appMap["app_name"].(string)
			if appName == "" {
				appName, _ = appMap["name"].(string)
			}
			if appName == "" {
				appName = pkgName
			}

			var mins int64
			if m, ok := appMap["minutes"].(float64); ok {
				mins = int64(m)
			}

			if _, exists := appTotals[pkgName]; !exists {
				appTotals[pkgName] = &AppUsageSummary{
					PackageName: pkgName,
					AppName:     appName,
				}
			}
			appTotals[pkgName].Minutes += mins
		}
	}

	// Resolve package names using DB / resolver
	pkgList := make([]string, 0, len(appTotals))
	for pkg := range appTotals {
		pkgList = append(pkgList, pkg)
	}

	var resolved map[string]models.AppMetadata
	if h.Resolver != nil {
		resolved = h.Resolver.GetResolvedNames(pkgList)
	} else {
		resolved = make(map[string]models.AppMetadata)
	}
	if resolved == nil {
		resolved = make(map[string]models.AppMetadata)
	}

	var aliases []models.AppAlias
	if len(pkgList) > 0 {
		h.DB.Where("user_id = ? AND package_name IN (?)", userID, pkgList).Find(&aliases)
		for _, a := range aliases {
			if meta, ok := resolved[a.PackageName]; ok {
				meta.AppName = a.AppName
				resolved[a.PackageName] = meta
			} else {
				resolved[a.PackageName] = models.AppMetadata{
					PackageName: a.PackageName,
					AppName:     a.AppName,
					Source:      "custom",
				}
			}
		}
	}

	result := make([]AppUsageSummary, 0, len(appTotals))
	for pkg, summary := range appTotals {
		if meta, found := resolved[pkg]; found && meta.AppName != "" {
			summary.AppName = meta.AppName
			summary.Category = meta.Category
		} else {
			// Not yet resolved locally: use humanized fallback immediately
			if summary.AppName == "" || summary.AppName == pkg {
				summary.AppName = resolver.FormatFallbackName(pkg)
			}
			if h.Resolver != nil {
				// Fire non-blocking background queue to fetch Play Store metadata
				h.Resolver.QueuePackage(pkg)
			}
		}
		result = append(result, *summary)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Minutes != result[j].Minutes {
			return result[i].Minutes > result[j].Minutes
		}
		return result[i].PackageName < result[j].PackageName
	})

	total := len(result)
	if offset > len(result) {
		result = []AppUsageSummary{}
	} else {
		end := offset + limit
		if end > len(result) {
			end = len(result)
		}
		result = result[offset:end]
	}

	return c.JSON(PaginatedAppsResponse{
		Total:  total,
		Offset: offset,
		Limit:  limit,
		Items:  result,
	})
}

type UpdateAppAliasRequest struct {
	PackageName string `json:"package_name"`
	AppName     string `json:"app_name"`
}

// SetAppAlias godoc
// @Summary      Set custom display name for an app package
// @Description  Overrides the app name displayed across dashboards
// @Tags         reports
// @Accept       json
// @Produce      json
// @Param        body body UpdateAppAliasRequest true "Alias details"
// @Success      200 {object} MessageResponse
// @Failure      400 {object} ErrorResponse
// @Router       /reports/screen-time/aliases [post]
func (h *ReportsHandler) SetAppAlias(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var req UpdateAppAliasRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	req.PackageName = strings.TrimSpace(req.PackageName)
	req.AppName = strings.TrimSpace(req.AppName)
	if req.PackageName == "" || req.AppName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "package_name and app_name are required"})
	}

	alias := models.AppAlias{
		UserID:      userID,
		PackageName: req.PackageName,
		AppName:     req.AppName,
	}
	err := h.DB.Where("user_id = ? AND package_name = ?", userID, req.PackageName).
		Assign(models.AppAlias{AppName: req.AppName}).
		FirstOrCreate(&alias).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to save alias"})
	}

	return c.JSON(MessageResponse{Message: "app name updated successfully"})
}

// GetHealthDailyTotals godoc
// @Summary      Get health daily totals
// @Description  Daily aggregated steps, distance, and calories
// @Tags         reports
// @Produce      json
// @Param        from query string false "From date YYYY-MM-DD"
// @Param        to query string false "To date YYYY-MM-DD"
// @Param        device_id query string false "Filter by device ID"
// @Success      200 {array} HealthDailySummary
// @Router       /reports/health/daily [get]
func (h *ReportsHandler) GetHealthDailyTotals(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	from := c.Query("from")
	to := c.Query("to")
	deviceIDParam := c.Query("device_id")

	deviceIDs, err := h.getUserDeviceIDs(userID, deviceIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: err.Error()})
	}
	if len(deviceIDs) == 0 {
		return c.JSON([]HealthDailySummary{})
	}

	query := h.DB.Where("type = 'daily_totals' AND device_id IN (?)", deviceIDs)
	if from != "" {
		query = query.Where("day >= ?", from)
	}
	if to != "" {
		query = query.Where("day <= ?", to)
	}

	var records []models.Record
	if err := query.Order("day ASC, seq DESC, built_at DESC").Find(&records).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to query records"})
	}

	// Deduplicate per (device_id, day)
	seen := make(map[string]bool)
	dayTotals := make(map[string]*HealthDailySummary)

	for _, r := range records {
		if r.Day == nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", r.DeviceID, *r.Day)
		if seen[key] {
			continue
		}
		seen[key] = true

		var dataMap map[string]interface{}
		_ = json.Unmarshal([]byte(r.Data), &dataMap)

		var steps int64
		if s, ok := dataMap["steps"].(float64); ok {
			steps = int64(s)
		}
		var distance float64
		if d, ok := dataMap["distance_meters"].(float64); ok {
			distance = d
		}
		var calories float64
		if c, ok := dataMap["active_calories"].(float64); ok {
			calories = c
		} else if c, ok := dataMap["calories"].(float64); ok {
			calories = c
		}

		dayStr := *r.Day
		if _, exists := dayTotals[dayStr]; !exists {
			dayTotals[dayStr] = &HealthDailySummary{Date: dayStr}
		}
		dayTotals[dayStr].Steps += steps
		dayTotals[dayStr].DistanceMeters += distance
		dayTotals[dayStr].Calories += calories
	}

	result := make([]HealthDailySummary, 0, len(dayTotals))
	for _, summary := range dayTotals {
		result = append(result, *summary)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return c.JSON(result)
}

// GetHealthTrends godoc
// @Summary      Get health activity trends
// @Description  Aggregates daily totals into weekly or monthly trends
// @Tags         reports
// @Produce      json
// @Param        period query string false "weekly or monthly (default weekly)"
// @Param        from query string false "From date YYYY-MM-DD"
// @Param        to query string false "To date YYYY-MM-DD"
// @Success      200 {array} TrendSummary
// @Router       /reports/health/trends [get]
func (h *ReportsHandler) GetHealthTrends(c fiber.Ctx) error {
	periodType := c.Query("period")
	if periodType != "monthly" {
		periodType = "weekly"
	}

	userID := c.Locals("userID").(uint)
	from := c.Query("from")
	to := c.Query("to")

	deviceIDs, err := h.getUserDeviceIDs(userID, "")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: err.Error()})
	}
	if len(deviceIDs) == 0 {
		return c.JSON([]TrendSummary{})
	}

	query := h.DB.Where("type = 'daily_totals' AND device_id IN (?)", deviceIDs)
	if from != "" {
		query = query.Where("day >= ?", from)
	}
	if to != "" {
		query = query.Where("day <= ?", to)
	}

	var records []models.Record
	if err := query.Order("day ASC, seq DESC, built_at DESC").Find(&records).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to query records"})
	}

	// First group by day across devices
	seen := make(map[string]bool)
	dayTotals := make(map[string]*HealthDailySummary)
	for _, r := range records {
		if r.Day == nil {
			continue
		}
		key := fmt.Sprintf("%d:%s", r.DeviceID, *r.Day)
		if seen[key] {
			continue
		}
		seen[key] = true

		var dataMap map[string]interface{}
		_ = json.Unmarshal([]byte(r.Data), &dataMap)

		var steps int64
		if s, ok := dataMap["steps"].(float64); ok {
			steps = int64(s)
		}
		var distance float64
		if d, ok := dataMap["distance_meters"].(float64); ok {
			distance = d
		}
		var calories float64
		if c, ok := dataMap["active_calories"].(float64); ok {
			calories = c
		} else if c, ok := dataMap["calories"].(float64); ok {
			calories = c
		}

		dayStr := *r.Day
		if _, exists := dayTotals[dayStr]; !exists {
			dayTotals[dayStr] = &HealthDailySummary{Date: dayStr}
		}
		dayTotals[dayStr].Steps += steps
		dayTotals[dayStr].DistanceMeters += distance
		dayTotals[dayStr].Calories += calories
	}

	// Now group into weekly/monthly periods
	periodMap := make(map[string]*TrendSummary)
	for dayStr, daySummary := range dayTotals {
		t, err := time.Parse("2006-01-02", dayStr)
		if err != nil {
			continue
		}

		var pKey string
		if periodType == "monthly" {
			pKey = t.Format("2006-01")
		} else {
			year, week := t.ISOWeek()
			pKey = fmt.Sprintf("%04d-W%02d", year, week)
		}

		if _, exists := periodMap[pKey]; !exists {
			periodMap[pKey] = &TrendSummary{Period: pKey}
		}
		periodMap[pKey].StepsTotal += daySummary.Steps
		periodMap[pKey].DistanceTotal += daySummary.DistanceMeters
		periodMap[pKey].CaloriesTotal += daySummary.Calories
		periodMap[pKey].DaysCount++
	}

	result := make([]TrendSummary, 0, len(periodMap))
	for _, s := range periodMap {
		if s.DaysCount > 0 {
			s.StepsAvg = float64(s.StepsTotal) / float64(s.DaysCount)
		}
		result = append(result, *s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Period < result[j].Period
	})

	return c.JSON(result)
}

// GetSleep godoc
// @Summary      Get sleep reports
// @Description  Sleep duration and sessions assigned to wake date
// @Tags         reports
// @Produce      json
// @Param        from query string false "From date YYYY-MM-DD"
// @Param        to query string false "To date YYYY-MM-DD"
// @Param        device_id query string false "Filter by device ID"
// @Success      200 {array} SleepSessionSummary
// @Router       /reports/health/sleep [get]
func (h *ReportsHandler) GetSleep(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	from := c.Query("from")
	to := c.Query("to")
	deviceIDParam := c.Query("device_id")

	deviceIDs, err := h.getUserDeviceIDs(userID, deviceIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: err.Error()})
	}
	if len(deviceIDs) == 0 {
		return c.JSON([]SleepSessionSummary{})
	}

	query := h.DB.Where("type = 'sleep' AND device_id IN (?)", deviceIDs)
	if from != "" {
		query = query.Where("day >= ?", from)
	}
	if to != "" {
		query = query.Where("day <= ?", to)
	}

	var records []models.Record
	if err := query.Order("ts ASC").Find(&records).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to query sleep"})
	}

	result := make([]SleepSessionSummary, 0, len(records))
	for _, r := range records {
		var dataMap map[string]interface{}
		_ = json.Unmarshal([]byte(r.Data), &dataMap)

		var durMinutes float64
		if d, ok := dataMap["duration_seconds"].(float64); ok {
			durMinutes = d / 60.0
		} else if d, ok := dataMap["duration_minutes"].(float64); ok {
			durMinutes = d
		}

		wakeDate := r.Ts.Format("2006-01-02")
		if r.Day != nil {
			wakeDate = *r.Day
		}

		st := r.Ts
		if r.StartTs != nil {
			st = *r.StartTs
		}

		result = append(result, SleepSessionSummary{
			RecordID:        r.RecordID,
			WakeDate:        wakeDate,
			DurationMinutes: durMinutes,
			StartTime:       st,
			EndTime:         r.Ts,
		})
	}

	return c.JSON(result)
}

// GetSyncStatus godoc
// @Summary      Get sync status across all devices
// @Description  Returns sync health indicators and stale flags
// @Tags         reports
// @Produce      json
// @Success      200 {object} SyncStatusResponse
// @Router       /reports/sync-status [get]
func (h *ReportsHandler) GetSyncStatus(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var devices []models.Device
	if err := h.DB.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "database error"})
	}

	now := time.Now().UTC()
	staleThreshold := 2 * time.Hour

	items := make([]SyncStatusItem, len(devices))
	for i, d := range devices {
		isStale := false
		if d.LastSyncAt == nil || now.Sub(*d.LastSyncAt) > staleThreshold {
			isStale = true
		}

		items[i] = SyncStatusItem{
			DeviceID:         d.ID,
			DisplayName:      d.DisplayName,
			DeviceModel:      d.DeviceModel,
			CollectorVersion: d.CollectorVersion,
			Status:           d.Status,
			LastSyncAt:       d.LastSyncAt,
			LastSyncSource:   d.LastSyncSource,
			LastSyncError:    d.LastSyncError,
			IsStale:          isStale,
		}
	}

	return c.JSON(SyncStatusResponse{Devices: items})
}

// ExportData godoc
// @Summary      Export user records
// @Description  Export usage and health data in CSV or JSON format
// @Tags         data
// @Produce      text/csv,application/json
// @Param        from query string false "From date YYYY-MM-DD"
// @Param        to query string false "To date YYYY-MM-DD"
// @Param        format query string false "csv or json (default json)"
// @Success      200 {string} string
// @Router       /data/export [get]
func (h *ReportsHandler) ExportData(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	from := c.Query("from")
	to := c.Query("to")
	format := strings.ToLower(c.Query("format"))

	deviceIDs, err := h.getUserDeviceIDs(userID, "")
	if err != nil || len(deviceIDs) == 0 {
		if format == "csv" {
			c.Set("Content-Type", "text/csv")
			return c.SendString("id,uuid,device_id,device_model,type,day,ts,source_app,data\n")
		}
		return c.JSON([]models.Record{})
	}

	query := h.DB.Where("device_id IN (?)", deviceIDs)
	if from != "" {
		query = query.Where("day >= ?", from)
	}
	if to != "" {
		query = query.Where("day <= ?", to)
	}

	var records []models.Record
	if err := query.Order("ts ASC").Find(&records).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "database error"})
	}

	if format == "csv" {
		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", "attachment; filename=life-dashboard-export.csv")

		var buf strings.Builder
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"id", "uuid", "device_id", "device_model", "type", "day", "ts", "source_app", "data"})

		for _, r := range records {
			dayStr := ""
			if r.Day != nil {
				dayStr = *r.Day
			}
			_ = w.Write([]string{
				strconv.FormatUint(uint64(r.ID), 10),
				r.UUID,
				strconv.FormatUint(uint64(r.DeviceID), 10),
				r.DeviceModel,
				r.Type,
				dayStr,
				r.Ts.Format(time.RFC3339),
				r.SourceApp,
				r.Data,
			})
		}
		w.Flush()
		return c.SendString(buf.String())
	}

	return c.JSON(records)
}

// DeleteData godoc
// @Summary      Delete user data within range
// @Description  Deletes records, payloads, buckets, and tombstones for specified scope
// @Tags         data
// @Param        from query string false "From date YYYY-MM-DD"
// @Param        to query string false "To date YYYY-MM-DD"
// @Param        device_id query string false "Device ID"
// @Success      200 {object} MessageResponse
// @Router       /data [delete]
func (h *ReportsHandler) DeleteData(c fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	from := c.Query("from")
	to := c.Query("to")
	deviceIDParam := c.Query("device_id")

	deviceIDs, err := h.getUserDeviceIDs(userID, deviceIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: err.Error()})
	}
	if len(deviceIDs) == 0 {
		return c.JSON(MessageResponse{Message: "no records found to delete"})
	}

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		buildQuery := func() *gorm.DB {
			q := tx.Where("device_id IN (?)", deviceIDs)
			if from != "" {
				q = q.Where("day >= ?", from)
			}
			if to != "" {
				q = q.Where("day <= ?", to)
			}
			return q
		}

		if err := buildQuery().Delete(&models.Record{}).Error; err != nil {
			return err
		}

		// Buckets use bucket_start (time.Time), not day — string dates work via DB driver coercion
		bucketQ := tx.Where("device_id IN (?)", deviceIDs)
		if from != "" {
			bucketQ = bucketQ.Where("bucket_start >= ?", from)
		}
		if to != "" {
			bucketQ = bucketQ.Where("bucket_start < ?", to+"T23:59:59Z")
		}
		if err := bucketQ.Delete(&models.Bucket{}).Error; err != nil {
			return err
		}

		// Tombstones and Payloads don't have a day column — delete by device scope only
		if from == "" && to == "" {
			if err := tx.Where("device_id IN (?)", deviceIDs).Delete(&models.Tombstone{}).Error; err != nil {
				return err
			}
			if err := tx.Where("device_id IN (?)", deviceIDs).Delete(&models.Payload{}).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to delete data"})
	}

	return c.JSON(MessageResponse{Message: "data deleted successfully"})
}
