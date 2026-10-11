package ingestion

import (
	"testing"
	"time"
)

func TestCompanionAppPayloadVariant(t *testing.T) {
	// Exact payload shape observed from Companion app:
	jsonStr := `{
		"source": "screen_time",
		"sequence": 43,
		"device": "samsung SM-S931B",
		"app_version": "1.23.0",
		"timestamp": "2026-10-11T05:37:19.285959Z",
		"screen_time": [
			{
				"date": "2026-10-11",
				"total_screen_time_minutes": 267,
				"apps": [
					{"package": "org.mozilla.fenix", "name": "fenix", "minutes": 51, "last_used": "2026-10-11T05:32:13.453Z"},
					{"package": "com.every_bits.pr_13", "name": "pr_13", "minutes": 51, "last_used": "2026-10-11T04:30:03.542Z"}
				]
			},
			{
				"date": "2026-10-10",
				"total_screen_time_minutes": 310,
				"apps": [
					{"package": "org.mozilla.fenix", "name": "fenix", "minutes": 100}
				]
			}
		]
	}`

	parsed, err := ParsePayload(1, []byte(jsonStr), time.UTC)
	if err != nil {
		t.Fatalf("failed to parse companion payload: %v", err)
	}

	if len(parsed.Records) != 2 {
		t.Fatalf("expected 2 days of records, got %d", len(parsed.Records))
	}

	r0 := parsed.Records[0]
	if *r0.Day != "2026-10-11" {
		t.Errorf("expected day 2026-10-11, got %s", *r0.Day)
	}
	r1 := parsed.Records[1]
	if *r1.Day != "2026-10-10" {
		t.Errorf("expected day 2026-10-10, got %s", *r1.Day)
	}
}
