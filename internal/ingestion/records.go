package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/awang-karisma/life-dashboard-hub/internal/models"
)

var envelopeKeys = map[string]bool{
	"source":                       true,
	"sequence":                     true,
	"app_version":                  true,
	"built_at":                     true,
	"kind":                         true,
	"ping":                         true,
	"deleted_records":              true,
	"_resolutions":                 true,
	"device":                       true,
	"time_zone":                    true,
	"export_time":                  true,
	"window_complete":              true,
	"total_screen_time_minutes":    true,
	"filtered_screen_time_minutes": true,
	"apps":                         true,
	"date":                         true,
	"days":                         true,
	"_app_filter":                  true,
}

type ParsedPayload struct {
	Source         string
	Sequence       *int64
	AppVersion     string
	BuiltAt        *time.Time
	Kind           string
	DeviceModel    string
	TimeZone       string
	IsPing         bool
	WindowComplete bool
	Records        []*models.Record
	Buckets        []*models.Bucket
	DeletedRecords []DeletedRecord
}

type DeletedRecord struct {
	Type string `json:"type"`
	UUID string `json:"uuid"`
}

func ParsePayload(deviceID uint, bodyBytes []byte, serverTZ *time.Location) (*ParsedPayload, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	result := &ParsedPayload{
		Kind: "sync",
	}

	if pingVal, ok := raw["ping"].(bool); ok && pingVal {
		result.IsPing = true
		result.Kind = "ping"
	} else if typeVal, ok := raw["type"].(string); ok && typeVal == "ping" {
		result.IsPing = true
		result.Kind = "ping"
	}

	if src, ok := raw["source"].(string); ok {
		result.Source = src
	}
	if seqVal, ok := raw["sequence"]; ok && seqVal != nil {
		if s, ok := seqVal.(float64); ok {
			seqInt := int64(s)
			result.Sequence = &seqInt
		}
	}
	if v, ok := raw["app_version"].(string); ok {
		result.AppVersion = v
	}
	if k, ok := raw["kind"].(string); ok && k != "" {
		result.Kind = k
	}
	if d, ok := raw["device"].(string); ok {
		result.DeviceModel = d
	}
	if tz, ok := raw["time_zone"].(string); ok {
		result.TimeZone = tz
	}
	if wc, ok := raw["window_complete"].(bool); ok {
		result.WindowComplete = wc
	}
	if bAt, ok := raw["built_at"].(string); ok {
		if t, err := parseTimestamp(bAt); err == nil {
			result.BuiltAt = &t
		}
	}

	loc := serverTZ
	if loc == nil {
		loc = time.UTC
	}

	now := time.Now().UTC()

	// 1. Process deletions
	if delList, ok := raw["deleted_records"].([]interface{}); ok {
		for _, item := range delList {
			switch v := item.(type) {
			case string:
				result.DeletedRecords = append(result.DeletedRecords, DeletedRecord{UUID: v})
			case map[string]interface{}:
				d := DeletedRecord{}
				if u, ok := v["uuid"].(string); ok {
					d.UUID = u
				} else if id, ok := v["id"].(string); ok {
					d.UUID = id
				}
				if t, ok := v["type"].(string); ok {
					d.Type = t
				}
				if d.UUID != "" {
					result.DeletedRecords = append(result.DeletedRecords, d)
				}
			}
		}
	}

	// 2. Process Screen Time payloads
	if result.Source == "screen_time" {
		result.Kind = "screen_time"
		// Companion app sends days either under "screen_time" or "days"
		var daysList []interface{}
		if stList, ok := raw["screen_time"].([]interface{}); ok && len(stList) > 0 {
			daysList = stList
		} else if dList, ok := raw["days"].([]interface{}); ok && len(dList) > 0 {
			daysList = dList
		}

		if len(daysList) > 0 {
			for _, dayObj := range daysList {
				if dayMap, ok := dayObj.(map[string]interface{}); ok {
					rec := parseScreenTimeDay(deviceID, result, dayMap, loc, now)
					if rec != nil {
						result.Records = append(result.Records, rec)
					}
				}
			}
		} else {
			// Single day in root payload
			rec := parseScreenTimeDay(deviceID, result, raw, loc, now)
			if rec != nil {
				result.Records = append(result.Records, rec)
			}
		}
	}

	// 3. Process Health Connect / other payloads
	if result.Source == "health_connect" || (result.Source != "screen_time" && result.Source != "") {
		// Daily totals
		if dtVal, ok := raw["daily_totals"]; ok {
			switch dt := dtVal.(type) {
			case []interface{}:
				for _, item := range dt {
					if itemMap, ok := item.(map[string]interface{}); ok {
						rec := parseDailyTotal(deviceID, result, itemMap, loc, now)
						if rec != nil {
							result.Records = append(result.Records, rec)
						}
					}
				}
			case map[string]interface{}:
				rec := parseDailyTotal(deviceID, result, dt, loc, now)
				if rec != nil {
					result.Records = append(result.Records, rec)
				}
			}
		}

		// Other record types (keys that are not envelope keys)
		for k, val := range raw {
			if envelopeKeys[k] || k == "daily_totals" {
				continue
			}
			recList, ok := val.([]interface{})
			if !ok {
				continue
			}
			for _, item := range recList {
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				rec := parseHealthRecord(deviceID, result, k, itemMap, loc, now)
				if rec != nil {
					result.Records = append(result.Records, rec)
				}
			}
		}

		// Resolutions / Buckets
		if resMap, ok := raw["_resolutions"].(map[string]interface{}); ok {
			for typeName, bucketsVal := range resMap {
				bList, ok := bucketsVal.([]interface{})
				if !ok {
					continue
				}
				for _, bItem := range bList {
					bMap, ok := bItem.(map[string]interface{})
					if !ok {
						continue
					}
					bucket := parseBucket(deviceID, result, typeName, bMap, now)
					if bucket != nil {
						result.Buckets = append(result.Buckets, bucket)
					}
				}
			}
		}
	}

	return result, nil
}

func parseScreenTimeDay(deviceID uint, payload *ParsedPayload, dayMap map[string]interface{}, loc *time.Location, now time.Time) *models.Record {
	dateStr, _ := dayMap["date"].(string)
	if dateStr == "" {
		if payload.BuiltAt != nil {
			dateStr = payload.BuiltAt.In(loc).Format("2006-01-02")
		} else {
			dateStr = now.In(loc).Format("2006-01-02")
		}
	}

	uuid := fmt.Sprintf("screen_time:%d:%s:%s:%s", deviceID, payload.Source, payload.DeviceModel, dateStr)

	// Preserve app filter info if present
	dataBytes, _ := json.Marshal(dayMap)

	ts := now
	if t, err := time.ParseInLocation("2006-01-02", dateStr, loc); err == nil {
		ts = t
	}
	if payload.BuiltAt != nil {
		ts = *payload.BuiltAt
	}

	return &models.Record{
		Type:          "screen_time",
		Ts:            ts,
		UUID:          uuid,
		SourceApp:     "companion",
		PayloadSource: payload.Source,
		ReceivedAt:    now,
		Data:          string(dataBytes),
		DeviceID:      deviceID,
		Seq:           payload.Sequence,
		BuiltAt:       payload.BuiltAt,
		Day:           &dateStr,
		DeviceModel:   payload.DeviceModel,
		StartTs:       &ts,
		RecordID:      uuid,
	}
}

func parseDailyTotal(deviceID uint, payload *ParsedPayload, dtMap map[string]interface{}, loc *time.Location, now time.Time) *models.Record {
	dateStr, _ := dtMap["date"].(string)
	if dateStr == "" {
		if payload.BuiltAt != nil {
			dateStr = payload.BuiltAt.In(loc).Format("2006-01-02")
		} else {
			dateStr = now.In(loc).Format("2006-01-02")
		}
	}

	uuid := fmt.Sprintf("daily_totals:%d:%s:%s:%s", deviceID, payload.Source, payload.DeviceModel, dateStr)
	dataBytes, _ := json.Marshal(dtMap)

	ts := now
	if t, err := time.ParseInLocation("2006-01-02", dateStr, loc); err == nil {
		ts = t
	}
	if payload.BuiltAt != nil {
		ts = *payload.BuiltAt
	}

	return &models.Record{
		Type:          "daily_totals",
		Ts:            ts,
		UUID:          uuid,
		SourceApp:     "companion",
		PayloadSource: payload.Source,
		ReceivedAt:    now,
		Data:          string(dataBytes),
		DeviceID:      deviceID,
		Seq:           payload.Sequence,
		BuiltAt:       payload.BuiltAt,
		Day:           &dateStr,
		DeviceModel:   payload.DeviceModel,
		StartTs:       &ts,
		RecordID:      uuid,
	}
}

func parseHealthRecord(deviceID uint, payload *ParsedPayload, recType string, itemMap map[string]interface{}, loc *time.Location, now time.Time) *models.Record {
	// UUID
	var uuid string
	if u, ok := itemMap["uuid"].(string); ok && u != "" {
		uuid = u
	} else if id, ok := itemMap["id"].(string); ok && id != "" {
		uuid = id
	}

	dataBytes, _ := json.Marshal(itemMap)

	if uuid == "" {
		// Generate synthetic UUID
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%d:%s:%s", deviceID, recType, string(dataBytes))))
		uuid = hex.EncodeToString(h.Sum(nil))
	}

	// Timestamps
	ts := now
	var startTs *time.Time

	if tStr, ok := itemMap["time"].(string); ok {
		if t, err := parseTimestamp(tStr); err == nil {
			ts = t
		}
	} else if tStr, ok := itemMap["end_time"].(string); ok {
		if t, err := parseTimestamp(tStr); err == nil {
			ts = t
		}
	} else if tStr, ok := itemMap["session_end_time"].(string); ok {
		if t, err := parseTimestamp(tStr); err == nil {
			ts = t
		}
	}

	if stStr, ok := itemMap["start_time"].(string); ok {
		if t, err := parseTimestamp(stStr); err == nil {
			startTs = &t
		}
	} else if durSec, ok := itemMap["duration_seconds"].(float64); ok {
		st := ts.Add(-time.Duration(durSec * float64(time.Second)))
		startTs = &st
	} else {
		startTs = &ts
	}

	var sourceApp string
	if sa, ok := itemMap["source_app"].(string); ok {
		sourceApp = sa
	} else if pName, ok := itemMap["package_name"].(string); ok {
		sourceApp = pName
	}

	dayStr := ts.In(loc).Format("2006-01-02")
	recordID := strings.SplitN(uuid, "#", 2)[0]

	return &models.Record{
		Type:          recType,
		Ts:            ts,
		UUID:          uuid,
		SourceApp:     sourceApp,
		PayloadSource: payload.Source,
		ReceivedAt:    now,
		Data:          string(dataBytes),
		DeviceID:      deviceID,
		Seq:           payload.Sequence,
		BuiltAt:       payload.BuiltAt,
		Day:           &dayStr,
		DeviceModel:   payload.DeviceModel,
		StartTs:       startTs,
		RecordID:      recordID,
	}
}

func parseBucket(deviceID uint, payload *ParsedPayload, typeName string, bMap map[string]interface{}, now time.Time) *models.Bucket {
	startStr, _ := bMap["bucket_start"].(string)
	endStr, _ := bMap["bucket_end"].(string)
	if startStr == "" || endStr == "" {
		return nil
	}

	bStart, err := parseTimestamp(startStr)
	if err != nil {
		return nil
	}
	bEnd, err := parseTimestamp(endStr)
	if err != nil {
		return nil
	}

	bucket := &models.Bucket{
		DeviceID:      deviceID,
		Type:          typeName,
		PayloadSource: payload.Source,
		BucketStart:   bStart,
		BucketEnd:     bEnd,
		Complete:      payload.WindowComplete,
		Seq:           payload.Sequence,
		BuiltAt:       payload.BuiltAt,
		ReceivedAt:    now,
	}

	if cmp, ok := bMap["complete"].(bool); ok {
		bucket.Complete = cmp
	}

	if sc, ok := bMap["sample_count"].(float64); ok {
		c := int64(sc)
		bucket.SampleCount = &c
	}
	if v, ok := bMap["avg"].(float64); ok {
		bucket.Avg = &v
	}
	if v, ok := bMap["min"].(float64); ok {
		bucket.Min = &v
	}
	if v, ok := bMap["max"].(float64); ok {
		bucket.Max = &v
	}
	if v, ok := bMap["total"].(float64); ok {
		bucket.Total = &v
	}

	if srcList, ok := bMap["sources"].([]interface{}); ok {
		srcBytes, _ := json.Marshal(srcList)
		bucket.Sources = string(srcBytes)
	}

	return bucket
}

func parseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}
