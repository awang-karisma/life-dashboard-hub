package resolver

import (
	"testing"
)

func TestPlayStoreScraper(t *testing.T) {
	pr := NewPackageResolver(nil)
	name, err := pr.fetchPlayStoreName("com.habitrpg.android.habitica")
	if err != nil {
		t.Fatalf("fetchPlayStoreName failed: %v", err)
	}

	t.Logf("Resolved name: %s", name)
	if name != "Habitica: Gamify Your Tasks" {
		t.Errorf("expected 'Habitica: Gamify Your Tasks', got '%s'", name)
	}
}
