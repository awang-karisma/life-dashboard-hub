package resolver

import (
	"testing"
)

func TestPlayStoreScraper(t *testing.T) {
	pr := NewPackageResolver(nil)
	result, err := pr.fetchPlayStoreName("com.habitrpg.android.habitica")
	if err != nil {
		t.Fatalf("fetchPlayStoreName failed: %v", err)
	}
	t.Logf("Resolved name: %s, category: %s", result.Name, result.Category)
	if result.Name != "Habitica: Gamify Your Tasks" {
		t.Errorf("expected 'Habitica: Gamify Your Tasks', got '%s'", result.Name)
	}
	if result.Category != "PRODUCTIVITY" {
		t.Errorf("expected category 'PRODUCTIVITY', got '%s'", result.Category)
	}
}
