package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestIngestionBusPubSub(t *testing.T) {
	bus := NewIngestionBus()

	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	event := IngestionEvent{
		Timestamp:  time.Now(),
		UserID:     1,
		DeviceID:   1,
		DeviceName: "Pixel 8",
		Source:     "companion-app",
		SHA256:     "abcd1234efgh5678",
		Records:    10,
		Tombstones: 2,
		LatencyMs:  45,
		Status:     "PAYLOAD_OK",
	}

	bus.Publish(event)

	select {
	case received := <-ch:
		if received.DeviceID != event.DeviceID {
			t.Errorf("expected DeviceID %d, got %d", event.DeviceID, received.DeviceID)
		}
		if received.Status != "PAYLOAD_OK" {
			t.Errorf("expected Status PAYLOAD_OK, got %s", received.Status)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event on bus channel")
	}

	recent := bus.Recent()
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent event, got %d", len(recent))
	}
	if recent[0].DeviceName != "Pixel 8" {
		t.Errorf("expected DeviceName Pixel 8, got %s", recent[0].DeviceName)
	}
}

func TestStreamIngestionHeadersAndHydration(t *testing.T) {
	bus := NewIngestionBus()

	// Seed 1 event
	bus.Publish(IngestionEvent{
		Timestamp:  time.Now(),
		UserID:     1,
		DeviceID:   2,
		DeviceName: "Galaxy Tab",
		Source:     "health_connect",
		SHA256:     "1234567890abcdef",
		Records:    5,
		Tombstones: 0,
		LatencyMs:  12,
		Status:     "PAYLOAD_OK",
	})

	app := fiber.New()
	handler := NewEventsHandler(bus)
	app.Get("/events/ingestion", func(c fiber.Ctx) error {
		c.Locals("userID", uint(1))
		return c.Next()
	}, handler.StreamIngestion)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/events/ingestion", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream content type, got: %s", ct)
	}

	// Read first SSE data line
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read first line: %v", err)
	}

	if !strings.HasPrefix(line, "data: ") {
		t.Errorf("expected data line prefix, got: %s", line)
	}

	jsonStr := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
	var ev IngestionEvent
	if err := json.Unmarshal([]byte(jsonStr), &ev); err != nil {
		t.Fatalf("failed to unmarshal SSE event: %v", err)
	}

	if ev.DeviceName != "Galaxy Tab" {
		t.Errorf("expected Galaxy Tab, got: %s", ev.DeviceName)
	}
}

func TestStreamIngestionUserScoping(t *testing.T) {
	bus := NewIngestionBus()

	// Publish events for user 1 and user 2
	bus.Publish(IngestionEvent{
		Timestamp:  time.Now(),
		UserID:     1,
		DeviceID:   10,
		DeviceName: "User 1 Phone",
		Status:     "PAYLOAD_OK",
	})
	bus.Publish(IngestionEvent{
		Timestamp:  time.Now(),
		UserID:     2,
		DeviceID:   20,
		DeviceName: "User 2 Phone",
		Status:     "PAYLOAD_OK",
	})

	app := fiber.New()
	handler := NewEventsHandler(bus)
	app.Get("/events/ingestion", func(c fiber.Ctx) error {
		c.Locals("userID", uint(1))
		return c.Next()
	}, handler.StreamIngestion)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/events/ingestion", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read line: %v", err)
	}

	jsonStr := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
	var ev IngestionEvent
	if err := json.Unmarshal([]byte(jsonStr), &ev); err != nil {
		t.Fatalf("failed to unmarshal SSE event: %v", err)
	}

	if ev.UserID != 1 || ev.DeviceName != "User 1 Phone" {
		t.Errorf("expected User 1 Phone event for user 1, got %+v", ev)
	}
}
