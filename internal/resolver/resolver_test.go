package resolver

import (
	"sync"
	"testing"
	"time"

	"github.com/owen282000/life-dashboard-hub/internal/database"
	"github.com/owen282000/life-dashboard-hub/internal/models"
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

	// Wait briefly for background worker to process the single item
	time.Sleep(600 * time.Millisecond)

	// Verify only 1 entry in DB
	var records []models.AppMetadata
	if err := db.Where("package_name = ?", pkg).Find(&records).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected exactly 1 DB record, got %d", len(records))
	}

	if records[0].AppName != "Uniqueapp" {
		t.Errorf("expected fallback 'Uniqueapp', got '%s'", records[0].AppName)
	}

	// Test user custom override
	if err := pr.SetCustomOverride(pkg, "My Favorite App"); err != nil {
		t.Fatalf("SetCustomOverride failed: %v", err)
	}

	resolved := pr.GetResolvedNames([]string{pkg})
	if resolved[pkg].AppName != "My Favorite App" || resolved[pkg].Source != "custom" {
		t.Errorf("expected custom override 'My Favorite App', got '%s'", resolved[pkg].AppName)
	}
}
