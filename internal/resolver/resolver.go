package resolver

import (
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
	"gorm.io/gorm"
)

var nameRegex = regexp.MustCompile(`itemprop="name"[^>]*>([^<]+)<`)
var fallbackNameRegex = regexp.MustCompile(`<h1[^>]*>.*?<span[^>]*>([^<]+)</span>.*?</h1>`)

// Known common system packages fallback
var commonPackages = map[string]string{
	"com.android.chrome":                "Google Chrome",
	"com.google.android.youtube":        "YouTube",
	"com.google.android.apps.maps":      "Google Maps",
	"com.google.android.apps.photos":    "Google Photos",
	"com.google.android.gm":             "Gmail",
	"com.google.android.apps.messaging": "Messages",
	"com.google.android.dialer":         "Phone",
	"com.whatsapp":                      "WhatsApp",
	"com.instagram.android":             "Instagram",
	"com.spotify.music":                 "Spotify",
	"org.mozilla.fenix":                 "Firefox",
	"org.mozilla.firefox":               "Firefox",
	"com.twitter.android":               "X (Twitter)",
	"com.zhiliaoapp.musically":          "TikTok",
	"org.telegram.messenger":            "Telegram",
	"com.discord":                       "Discord",
	"com.reddit.frontpage":              "Reddit",
	"com.netflix.mediaclient":           "Netflix",
	"com.amazon.mShop.android.shopping": "Amazon",
}

type PackageResolver struct {
	db         *database.DB
	httpClient *http.Client
	queue      chan string
	inFlight   sync.Map
}

func NewPackageResolver(db *database.DB) *PackageResolver {
	pr := &PackageResolver{
		db: db,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
		queue: make(chan string, 1000),
	}

	// Start background worker
	go pr.worker(context.Background())

	return pr
}

// QueuePackage queues a package name for background resolution if not already queued or in DB.
// Exactly one resolution runs per unique package name, regardless of how many concurrent users request it.
func (pr *PackageResolver) QueuePackage(pkg string) {
	pkg = strings.TrimSpace(pkg)
	if pkg == "" {
		return
	}

	// 1. In-memory check: return immediately if already queued, in-flight, or completed in this process
	if _, exists := pr.inFlight.LoadOrStore(pkg, true); exists {
		return
	}

	// 2. Database check: don't queue if already resolved in DB
	var count int64
	if err := pr.db.Model(&models.AppMetadata{}).Where("package_name = ?", pkg).Count(&count).Error; err == nil && count > 0 {
		return
	}

	select {
	case pr.queue <- pkg:
	default:
		// Queue full, drop but keep inFlight marker so we don't spam the channel
	}
}

func (pr *PackageResolver) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case pkg := <-pr.queue:
			pr.resolvePackage(pkg)
			// Keep pkg marked in pr.inFlight so subsequent calls by other users never re-trigger resolution
			time.Sleep(300 * time.Millisecond)
		}
	}
}

func (pr *PackageResolver) resolvePackage(pkg string) {
	// 1. Check known built-in dictionary
	if name, ok := commonPackages[pkg]; ok {
		meta := models.AppMetadata{
			PackageName: pkg,
			AppName:     name,
			Source:      "builtin",
			ResolvedAt:  time.Now().UTC(),
		}
		_ = pr.db.Save(&meta).Error
		return
	}

	// 2. Fetch from Google Play Store
	appName, err := pr.fetchPlayStoreName(pkg)
	if err == nil && appName != "" {
		meta := models.AppMetadata{
			PackageName: pkg,
			AppName:     appName,
			Source:      "play_store",
			ResolvedAt:  time.Now().UTC(),
		}
		_ = pr.db.Save(&meta).Error
		slog.Info("Resolved package name from Play Store", "package", pkg, "name", appName)
		return
	}
	// 3. Fallback: Pretty-print last package segment
	fallback := FormatFallbackName(pkg)
	meta := models.AppMetadata{
		PackageName: pkg,
		AppName:     fallback,
		Source:      "fallback",
		ResolvedAt:  time.Now().UTC(),
	}
	_ = pr.db.Save(&meta).Error
}

func (pr *PackageResolver) fetchPlayStoreName(pkg string) (string, error) {
	url := fmt.Sprintf("https://play.google.com/store/apps/details?id=%s&hl=en", pkg)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := pr.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	bodyStr := string(bodyBytes)

	m := nameRegex.FindStringSubmatch(bodyStr)
	if len(m) > 1 {
		return strings.TrimSpace(html.UnescapeString(m[1])), nil
	}

	m2 := fallbackNameRegex.FindStringSubmatch(bodyStr)
	if len(m2) > 1 {
		return strings.TrimSpace(html.UnescapeString(m2[1])), nil
	}

	return "", fmt.Errorf("name element not found in HTML")
}

// GetResolvedNames returns a map of package_name -> app_name for requested packages
func (pr *PackageResolver) GetResolvedNames(packages []string) map[string]models.AppMetadata {
	result := make(map[string]models.AppMetadata)
	if len(packages) == 0 {
		return result
	}

	var metadataList []models.AppMetadata
	_ = pr.db.Where("package_name IN (?)", packages).Find(&metadataList).Error

	for _, m := range metadataList {
		result[m.PackageName] = m
	}
	return result
}

// SetCustomOverride allows user to rename an app
func (pr *PackageResolver) SetCustomOverride(pkg, customName string) error {
	pkg = strings.TrimSpace(pkg)
	customName = strings.TrimSpace(customName)
	if pkg == "" || customName == "" {
		return fmt.Errorf("package_name and app_name are required")
	}

	meta := models.AppMetadata{
		PackageName: pkg,
		AppName:     customName,
		Source:      "custom",
		ResolvedAt:  time.Now().UTC(),
	}

	return pr.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(&meta).Error
}

func FormatFallbackName(pkg string) string {
	parts := strings.Split(pkg, ".")
	last := parts[len(parts)-1]
	last = strings.ReplaceAll(last, "_", " ")
	last = strings.ReplaceAll(last, "-", " ")
	words := strings.Fields(last)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	res := strings.Join(words, " ")
	if res == "" {
		return pkg
	}
	return res
}
