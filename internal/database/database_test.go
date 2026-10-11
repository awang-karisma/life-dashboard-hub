package database

import (
	"testing"
	"time"

	"github.com/owen282000/life-dashboard-hub/internal/models"
)

func TestDatabaseInitAndModels(t *testing.T) {
	db, err := OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	// Verify tables were created
	tables := []string{"users", "devices", "payloads", "records", "tombstones", "buckets", "families", "family_memberships", "invitations", "sessions"}
	for _, tbl := range tables {
		if !db.Migrator().HasTable(tbl) {
			t.Errorf("expected table %s to exist", tbl)
		}
	}
}

func TestUpsertRecordOrderingRule(t *testing.T) {
	db, err := OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	now := time.Now().UTC()
	t1 := now.Add(-10 * time.Minute)
	t2 := now.Add(-5 * time.Minute)
	t3 := now

	seq10 := int64(10)
	day := "2026-10-11"

	// 1. Insert record with seq=10, built_at=t2
	r1 := &models.Record{
		Type:          "screen_time",
		Ts:            now,
		UUID:          "test-uuid-1",
		SourceApp:     "companion",
		PayloadSource: "screen_time",
		Data:          `{"minutes": 100}`,
		DeviceID:      1,
		Seq:           &seq10,
		BuiltAt:       &t2,
		Day:           &day,
		DeviceModel:   "Pixel 8",
		RecordID:      "test-uuid-1",
	}

	affected, err := UpsertRecord(db.DB, db.Dialect, r1)
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 affected row, got %d", affected)
	}

	// Verify stored data
	var check models.Record
	if err := db.Where("uuid = ?", "test-uuid-1").First(&check).Error; err != nil {
		t.Fatalf("record not found: %v", err)
	}
	if check.Data != `{"minutes": 100}` {
		t.Fatalf("expected minutes: 100, got %s", check.Data)
	}

	// 2. Ingest payload with seq=5 and earlier built_at (t1) -> older payload must NOT overwrite
	seq5 := int64(5)
	rOlder := &models.Record{
		Type:          "screen_time",
		Ts:            now,
		UUID:          "test-uuid-1",
		SourceApp:     "companion",
		PayloadSource: "screen_time",
		Data:          `{"minutes": 50}`,
		DeviceID:      1,
		Seq:           &seq5,
		BuiltAt:       &t1,
		Day:           &day,
		DeviceModel:   "Pixel 8",
		RecordID:      "test-uuid-1",
	}

	affected, err = UpsertRecord(db.DB, db.Dialect, rOlder)
	if err != nil {
		t.Fatalf("older upsert failed: %v", err)
	}
	if affected != 0 {
		t.Fatalf("expected 0 affected rows for older payload, got %d", affected)
	}

	// Data should still be 100
	if err := db.Where("uuid = ?", "test-uuid-1").First(&check).Error; err != nil {
		t.Fatalf("record not found: %v", err)
	}
	if check.Data != `{"minutes": 100}` {
		t.Fatalf("expected data still 100, got %s", check.Data)
	}

	// 3. Ingest with seq=5 but NEWER built_at (t3) (e.g. app reinstall) -> MUST overwrite
	rNewer := &models.Record{
		Type:          "screen_time",
		Ts:            now,
		UUID:          "test-uuid-1",
		SourceApp:     "companion",
		PayloadSource: "screen_time",
		Data:          `{"minutes": 200}`,
		DeviceID:      1,
		Seq:           &seq5,
		BuiltAt:       &t3,
		Day:           &day,
		DeviceModel:   "Pixel 8",
		RecordID:      "test-uuid-1",
	}

	affected, err = UpsertRecord(db.DB, db.Dialect, rNewer)
	if err != nil {
		t.Fatalf("newer upsert failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 affected row for newer built_at, got %d", affected)
	}

	if err := db.Where("uuid = ?", "test-uuid-1").First(&check).Error; err != nil {
		t.Fatalf("record not found: %v", err)
	}
	if check.Data != `{"minutes": 200}` {
		t.Fatalf("expected updated minutes: 200, got %s", check.Data)
	}
}

func TestUpsertBucketLogic(t *testing.T) {
	db, err := OpenTestDB("")
	if err != nil {
		t.Fatalf("OpenTestDB failed: %v", err)
	}

	bStart := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	bEnd := time.Date(2026, 10, 11, 11, 0, 0, 0, time.UTC)
	cnt1 := int64(10)
	avg1 := 60.0
	min1 := 50.0
	max1 := 70.0
	tot1 := 600.0

	b1 := &models.Bucket{
		DeviceID:      1,
		Type:          "heart_rate",
		PayloadSource: "health_connect",
		BucketStart:   bStart,
		BucketEnd:     bEnd,
		SampleCount:   &cnt1,
		Avg:           &avg1,
		Min:           &min1,
		Max:           &max1,
		Total:         &tot1,
		Complete:      false,
		Sources:       `["app.one"]`,
	}

	if err := UpsertBucket(db.DB, b1); err != nil {
		t.Fatalf("UpsertBucket failed: %v", err)
	}

	// Merge second incomplete window
	cnt2 := int64(10)
	avg2 := 80.0
	min2 := 40.0
	max2 := 90.0
	tot2 := 800.0
	b2 := &models.Bucket{
		DeviceID:      1,
		Type:          "heart_rate",
		PayloadSource: "health_connect",
		BucketStart:   bStart,
		BucketEnd:     bEnd,
		SampleCount:   &cnt2,
		Avg:           &avg2,
		Min:           &min2,
		Max:           &max2,
		Total:         &tot2,
		Complete:      false,
		Sources:       `["app.two"]`,
	}

	if err := UpsertBucket(db.DB, b2); err != nil {
		t.Fatalf("UpsertBucket 2 failed: %v", err)
	}

	var stored models.Bucket
	if err := db.Where("device_id = 1 AND type = 'heart_rate' AND bucket_start = ?", bStart).First(&stored).Error; err != nil {
		t.Fatalf("failed to query bucket: %v", err)
	}

	if *stored.SampleCount != 20 {
		t.Errorf("expected sample_count 20, got %d", *stored.SampleCount)
	}
	if *stored.Avg != 70.0 {
		t.Errorf("expected avg 70.0, got %f", *stored.Avg)
	}
	if *stored.Min != 40.0 {
		t.Errorf("expected min 40.0, got %f", *stored.Min)
	}
	if *stored.Max != 90.0 {
		t.Errorf("expected max 90.0, got %f", *stored.Max)
	}
	if *stored.Total != 1400.0 {
		t.Errorf("expected total 1400.0, got %f", *stored.Total)
	}
}
