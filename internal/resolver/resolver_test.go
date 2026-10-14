package resolver

import (
	"sync"
	"testing"
	"time"

	"github.com/awang-karisma/life-dashboard-hub/internal/database"
	"github.com/awang-karisma/life-dashboard-hub/internal/models"
)

func TestPackageResolverSingleFlightAndFallback(t *testing.T) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	pr := NewPackageResolver(db)

	pkg := "com.example.uniqueapp"

	// Queue the same package 50 times concurrently from simulated multiple users
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pr.QueuePackage(pkg)
		}()
	}
	wg.Wait()

	// Wait for background worker to process the single item
	var records []models.AppMetadata
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		records = nil
		if err := db.Where("package_name = ?", pkg).Find(&records).Error; err == nil && len(records) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly 1 DB record, got %d", len(records))
	}

	if records[0].AppName != "Uniqueapp" {
		t.Errorf("expected fallback 'Uniqueapp', got '%s'", records[0].AppName)
	}

}
func TestResolverCategoryAndOverride(t *testing.T) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	pr := NewPackageResolver(db)
	pkg := "com.spotify.music"

	pr.resolvePackage(pkg)

	resolved := pr.GetResolvedNames([]string{pkg})
	meta, ok := resolved[pkg]
	if !ok {
		t.Fatalf("expected package %s to be resolved", pkg)
	}
	if meta.AppName != "Spotify" {
		t.Errorf("expected 'Spotify', got '%s'", meta.AppName)
	}
	if meta.Category != "MUSIC_AND_AUDIO" {
		t.Errorf("expected 'MUSIC_AND_AUDIO', got '%s'", meta.Category)
	}
	if meta.Source != "builtin" {
		t.Errorf("expected source 'builtin', got '%s'", meta.Source)
	}

}
