package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadValidation(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when SESSION_SECRET is missing, got nil")
	}
}

func TestConfigDefaultsAndEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("SESSION_SECRET", "test-secret-123")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SessionSecret != "test-secret-123" {
		t.Errorf("expected SESSION_SECRET 'test-secret-123', got '%s'", cfg.SessionSecret)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("expected default DataDir './data', got '%s'", cfg.DataDir)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("expected default ListenAddr ':8080', got '%s'", cfg.ListenAddr)
	}
	if cfg.TZ != "UTC" {
		t.Errorf("expected default TZ 'UTC', got '%s'", cfg.TZ)
	}
	if cfg.MaxBodyBytes != 32*1024*1024 {
		t.Errorf("expected default MaxBodyBytes 32MB, got %d", cfg.MaxBodyBytes)
	}
}

func TestConfigPrecedence(t *testing.T) {
	// Create temporary directory for config.yaml
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Write config.yaml
	yamlContent := "SESSION_SECRET: from-yaml\nLISTEN_ADDR: :9090\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Case 1: config.yaml provides SESSION_SECRET
	os.Clearenv()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading from yaml: %v", err)
	}
	if cfg.SessionSecret != "from-yaml" {
		t.Errorf("expected 'from-yaml', got '%s'", cfg.SessionSecret)
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("expected ':9090', got '%s'", cfg.ListenAddr)
	}

	// Case 2: Env var overrides config.yaml
	os.Setenv("SESSION_SECRET", "override-secret")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("unexpected error with env override: %v", err)
	}
	if cfg.SessionSecret != "override-secret" {
		t.Errorf("expected 'override-secret', got '%s'", cfg.SessionSecret)
	}
	// LISTEN_ADDR still comes from yaml
	if cfg.ListenAddr != ":9090" {
		t.Errorf("expected ':9090', got '%s'", cfg.ListenAddr)
	}
}
