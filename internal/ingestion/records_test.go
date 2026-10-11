package ingestion

import (
	"testing"
	"time"
)

func TestParsePayloadScreenTime(t *testing.T) {
	jsonStr := `{
		"source": "screen_time",
		"sequence": 42,
		"device": "Pixel Tablet",
		"app_version": "1.23.0",
		"built_at": "2026-10-11T08:30:00Z",
		"days": [
			{
				"date": "2026-10-10",
				"total_screen_time_minutes": 240,
				"apps": [
					{"package_name": "com.youtube", "app_name": "YouTube", "minutes": 180}
				]
			}
		]
	}`

	parsed, err := ParsePayload(5, []byte(jsonStr), time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Source != "screen_time" {
		t.Errorf("expected source screen_time, got %s", parsed.Source)
	}
	if parsed.Sequence == nil || *parsed.Sequence != 42 {
		t.Errorf("expected sequence 42, got %v", parsed.Sequence)
	}
	if parsed.DeviceModel != "Pixel Tablet" {
		t.Errorf("expected device model 'Pixel Tablet', got %s", parsed.DeviceModel)
	}
	if len(parsed.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(parsed.Records))
	}

	rec := parsed.Records[0]
	if rec.Type != "screen_time" {
		t.Errorf("expected type screen_time, got %s", rec.Type)
	}
	if rec.Day == nil || *rec.Day != "2026-10-10" {
		t.Errorf("expected day 2026-10-10, got %v", rec.Day)
	}
	if rec.UUID != "screen_time:5:screen_time:Pixel Tablet:2026-10-10" {
		t.Errorf("unexpected uuid: %s", rec.UUID)
	}
}

func TestParsePayloadHealthConnect(t *testing.T) {
	jsonStr := `{
		"source": "health_connect",
		"sequence": 15,
		"built_at": "2026-10-11T08:30:00Z",
		"daily_totals": [
			{"date": "2026-10-10", "steps": 8500}
		],
		"heart_rate": [
			{"uuid": "hr-rec-1#1728630000000", "time": "2026-10-10T12:00:00Z", "bpm": 72}
		],
		"deleted_records": [
			"deleted-uuid-1",
			{"uuid": "deleted-uuid-2", "type": "steps"}
		],
		"_resolutions": {
			"heart_rate": [
				{
					"bucket_start": "2026-10-10T12:00:00Z",
					"bucket_end": "2026-10-10T13:00:00Z",
					"sample_count": 60,
					"avg": 70.0
				}
			]
		}
	}`

	parsed, err := ParsePayload(2, []byte(jsonStr), time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Records) != 2 {
		t.Errorf("expected 2 records (1 daily_total + 1 heart_rate), got %d", len(parsed.Records))
	}
	if len(parsed.DeletedRecords) != 2 {
		t.Errorf("expected 2 deleted records, got %d", len(parsed.DeletedRecords))
	}
	if len(parsed.Buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(parsed.Buckets))
	}

	// Verify RecordID calculation (splitting on #)
	for _, r := range parsed.Records {
		if r.Type == "heart_rate" {
			if r.RecordID != "hr-rec-1" {
				t.Errorf("expected RecordID 'hr-rec-1', got '%s'", r.RecordID)
			}
		}
	}
}
