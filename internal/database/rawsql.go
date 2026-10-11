package database

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/awang-karisma/life-dashboard-hub/internal/models"
	"gorm.io/gorm"
)

// JSONExtract returns the dialect-specific SQL expression to extract a field from a JSON column.
func JSONExtract(dialect Dialect, col, path string) string {
	if dialect == DialectPostgres {
		return fmt.Sprintf("(%s::jsonb->>'%s')", col, path)
	}
	return fmt.Sprintf("json_extract(%s, '$.%s')", col, path)
}

// UpsertRecord inserts or updates a record respecting the sequence and built_at ordering guard.
// Returns rowsAffected (1 if inserted/updated, 0 if skipped due to older ordering).
func UpsertRecord(tx *gorm.DB, dialect Dialect, r *models.Record) (int64, error) {
	if r.ReceivedAt.IsZero() {
		r.ReceivedAt = time.Now().UTC()
	}

	var query string
	var args []any

	if dialect == DialectPostgres {
		query = `INSERT INTO records (type, ts, uuid, source_app, payload_source, received_at, data, device_id, seq, built_at, day, device_model, start_ts, record_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (uuid) DO UPDATE SET
  type = EXCLUDED.type,
  ts = EXCLUDED.ts,
  source_app = EXCLUDED.source_app,
  payload_source = EXCLUDED.payload_source,
  received_at = EXCLUDED.received_at,
  data = EXCLUDED.data,
  device_id = EXCLUDED.device_id,
  seq = EXCLUDED.seq,
  built_at = EXCLUDED.built_at,
  day = EXCLUDED.day,
  device_model = EXCLUDED.device_model,
  start_ts = EXCLUDED.start_ts,
  record_id = EXCLUDED.record_id
WHERE NOT (coalesce(EXCLUDED.seq < records.seq, true) AND coalesce(EXCLUDED.built_at < records.built_at, false))`

		args = []any{
			r.Type, r.Ts, r.UUID, r.SourceApp, r.PayloadSource, r.ReceivedAt, r.Data,
			r.DeviceID, r.Seq, r.BuiltAt, r.Day, r.DeviceModel, r.StartTs, r.RecordID,
		}
	} else {
		// SQLite dialect
		query = `INSERT INTO records (type, ts, uuid, source_app, payload_source, received_at, data, device_id, seq, built_at, day, device_model, start_ts, record_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (uuid) DO UPDATE SET
  type = EXCLUDED.type,
  ts = EXCLUDED.ts,
  source_app = EXCLUDED.source_app,
  payload_source = EXCLUDED.payload_source,
  received_at = EXCLUDED.received_at,
  data = EXCLUDED.data,
  device_id = EXCLUDED.device_id,
  seq = EXCLUDED.seq,
  built_at = EXCLUDED.built_at,
  day = EXCLUDED.day,
  device_model = EXCLUDED.device_model,
  start_ts = EXCLUDED.start_ts,
  record_id = EXCLUDED.record_id
WHERE NOT (coalesce(EXCLUDED.seq < records.seq, 1) AND coalesce(EXCLUDED.built_at < records.built_at, 0))`

		args = []any{
			r.Type, r.Ts, r.UUID, r.SourceApp, r.PayloadSource, r.ReceivedAt, r.Data,
			r.DeviceID, r.Seq, r.BuiltAt, r.Day, r.DeviceModel, r.StartTs, r.RecordID,
		}
	}

	res := tx.Exec(query, args...)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// UpsertTombstone inserts or updates a tombstone respecting ordering rules.
func UpsertTombstone(tx *gorm.DB, dialect Dialect, t *models.Tombstone) (int64, error) {
	if t.DeletedAt.IsZero() {
		t.DeletedAt = time.Now().UTC()
	}

	var query string
	var args []any

	if dialect == DialectPostgres {
		query = `INSERT INTO tombstones (type, uuid, device_id, seq, built_at, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (type, uuid) DO UPDATE SET
  device_id = EXCLUDED.device_id,
  seq = EXCLUDED.seq,
  built_at = EXCLUDED.built_at,
  deleted_at = EXCLUDED.deleted_at
WHERE NOT (coalesce(EXCLUDED.seq < tombstones.seq, true) AND coalesce(EXCLUDED.built_at < tombstones.built_at, false))`

		args = []any{t.Type, t.UUID, t.DeviceID, t.Seq, t.BuiltAt, t.DeletedAt}
	} else {
		query = `INSERT INTO tombstones (type, uuid, device_id, seq, built_at, deleted_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (type, uuid) DO UPDATE SET
  device_id = EXCLUDED.device_id,
  seq = EXCLUDED.seq,
  built_at = EXCLUDED.built_at,
  deleted_at = EXCLUDED.deleted_at
WHERE NOT (coalesce(EXCLUDED.seq < tombstones.seq, 1) AND coalesce(EXCLUDED.built_at < tombstones.built_at, 0))`

		args = []any{t.Type, t.UUID, t.DeviceID, t.Seq, t.BuiltAt, t.DeletedAt}
	}

	res := tx.Exec(query, args...)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// DeleteRecords deletes records matching uuid or record_id for a device if not prevented by ordering.
func DeleteRecords(tx *gorm.DB, dialect Dialect, deviceID uint, recType, uuid string, seq *int64, builtAt *time.Time) (int64, error) {
	var query string
	var args []any

	if dialect == DialectPostgres {
		query = `DELETE FROM records
WHERE device_id = $1 AND (uuid = $2 OR record_id = $2)
  AND NOT (coalesce($3 < seq, true) AND coalesce($4 < built_at, false))`
		args = []any{deviceID, uuid, seq, builtAt}
	} else {
		query = `DELETE FROM records
WHERE device_id = ? AND (uuid = ? OR record_id = ?)
  AND NOT (coalesce(? < seq, 1) AND coalesce(? < built_at, 0))`
		args = []any{deviceID, uuid, uuid, seq, builtAt}
	}

	res := tx.Exec(query, args...)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// UpsertBucket merges or replaces a bucket window inside the current transaction.
func UpsertBucket(tx *gorm.DB, b *models.Bucket) error {
	if b.ReceivedAt.IsZero() {
		b.ReceivedAt = time.Now().UTC()
	}

	var existing models.Bucket
	err := tx.Where(
		"device_id = ? AND type = ? AND payload_source = ? AND bucket_start = ?",
		b.DeviceID, b.Type, b.PayloadSource, b.BucketStart,
	).First(&existing).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return tx.Create(b).Error
		}
		return err
	}

	// Check ordering: if incoming seq < existing.seq AND incoming.built_at < existing.built_at, ignore incoming
	isOlderSeq := false
	if b.Seq != nil && existing.Seq != nil && *b.Seq < *existing.Seq {
		isOlderSeq = true
	} else if b.Seq == nil && existing.Seq != nil {
		isOlderSeq = true
	}

	isOlderBuiltAt := false
	if b.BuiltAt != nil && existing.BuiltAt != nil && b.BuiltAt.Before(*existing.BuiltAt) {
		isOlderBuiltAt = true
	} else if b.BuiltAt == nil && existing.BuiltAt != nil {
		isOlderBuiltAt = false
	}

	if isOlderSeq && isOlderBuiltAt {
		// Older payload, do not overwrite
		return nil
	}

	// Merge sources JSON string
	mergedSources := mergeSourcesJSON(existing.Sources, b.Sources)
	b.Sources = mergedSources

	if b.Complete {
		// Complete incoming window replaces aggregate values
		existing.BucketEnd = b.BucketEnd
		existing.SampleCount = b.SampleCount
		existing.Avg = b.Avg
		existing.Min = b.Min
		existing.Max = b.Max
		existing.Total = b.Total
		existing.Complete = true
		existing.Sources = b.Sources
		existing.Seq = b.Seq
		existing.BuiltAt = b.BuiltAt
		existing.ReceivedAt = b.ReceivedAt
		return tx.Save(&existing).Error
	}

	// Incoming window is incomplete
	if existing.Complete {
		// Don't downgrade complete bucket, but can update sources and metadata if newer
		existing.Sources = mergedSources
		return tx.Save(&existing).Error
	}

	// Both incomplete: combine statistics
	var c1, c2 int64
	if existing.SampleCount != nil {
		c1 = *existing.SampleCount
	}
	if b.SampleCount != nil {
		c2 = *b.SampleCount
	}
	totalCount := c1 + c2

	if totalCount > 0 {
		var avg1, avg2 float64
		if existing.Avg != nil {
			avg1 = *existing.Avg
		}
		if b.Avg != nil {
			avg2 = *b.Avg
		}
		mergedAvg := (avg1*float64(c1) + avg2*float64(c2)) / float64(totalCount)
		existing.Avg = &mergedAvg
		existing.SampleCount = &totalCount
	}

	// Min
	if b.Min != nil {
		if existing.Min == nil || *b.Min < *existing.Min {
			existing.Min = b.Min
		}
	}
	// Max
	if b.Max != nil {
		if existing.Max == nil || *b.Max > *existing.Max {
			existing.Max = b.Max
		}
	}
	// Total
	var t1, t2 float64
	hasTotal := false
	if existing.Total != nil {
		t1 = *existing.Total
		hasTotal = true
	}
	if b.Total != nil {
		t2 = *b.Total
		hasTotal = true
	}
	if hasTotal {
		sumTotal := t1 + t2
		existing.Total = &sumTotal
	}

	existing.Sources = mergedSources
	existing.Seq = b.Seq
	existing.BuiltAt = b.BuiltAt
	existing.ReceivedAt = b.ReceivedAt
	return tx.Save(&existing).Error
}

func mergeSourcesJSON(s1, s2 string) string {
	srcSet := make(map[string]struct{})
	var list1, list2 []string
	if s1 != "" {
		_ = json.Unmarshal([]byte(s1), &list1)
	}
	if s2 != "" {
		_ = json.Unmarshal([]byte(s2), &list2)
	}
	for _, s := range list1 {
		if s != "" {
			srcSet[s] = struct{}{}
		}
	}
	for _, s := range list2 {
		if s != "" {
			srcSet[s] = struct{}{}
		}
	}
	var merged []string
	for s := range srcSet {
		merged = append(merged, s)
	}
	b, _ := json.Marshal(merged)
	return string(b)
}
