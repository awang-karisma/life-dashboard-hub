package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/owen282000/life-dashboard-hub/internal/config"
	"github.com/owen282000/life-dashboard-hub/internal/database"
)

func TestCORSRestrictedToPublicURL(t *testing.T) {
	db, err := database.OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	cfg := &config.Config{
		PublicURL:    "https://app.example.com",
		MaxBodyBytes: 10 * 1024 * 1024,
	}

	srv := New(cfg, db)

	// 1. Request from untrusted origin -> should NOT allow evil origin
	reqEvil := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/info", nil)
	reqEvil.Header.Set("Origin", "https://evil.com")
	reqEvil.Header.Set("Access-Control-Request-Method", "GET")
	respEvil, err := srv.App.Test(reqEvil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	allowOrigin := respEvil.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin == "https://evil.com" {
		t.Errorf("expected evil origin to be rejected, but got Access-Control-Allow-Origin: %s", allowOrigin)
	}

	// 2. Request from configured PublicURL -> allowed
	reqGood := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/info", nil)
	reqGood.Header.Set("Origin", "https://app.example.com")
	reqGood.Header.Set("Access-Control-Request-Method", "GET")
	respGood, err := srv.App.Test(reqGood)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if got := respGood.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("expected https://app.example.com to be allowed, got: %s", got)
	}
}
