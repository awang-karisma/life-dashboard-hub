package models

import "time"

type Bucket struct {
	DeviceID      uint       `gorm:"primaryKey;not null" json:"device_id"`
	Type          string     `gorm:"primaryKey;not null" json:"type"`
	PayloadSource string     `gorm:"primaryKey;not null" json:"payload_source"`
	BucketStart   time.Time  `gorm:"primaryKey;not null" json:"bucket_start"`
	BucketEnd     time.Time  `gorm:"not null" json:"bucket_end"`
	SampleCount   *int64     `json:"sample_count"`
	Avg           *float64   `json:"avg"`
	Min           *float64   `json:"min"`
	Max           *float64   `json:"max"`
	Total         *float64   `json:"total"`
	Complete      bool       `gorm:"not null;default:false" json:"complete"`
	Sources       string     `gorm:"type:text" json:"sources"` // JSON array string e.g. '["com.fitbit"]'
	Seq           *int64     `json:"seq"`
	BuiltAt       *time.Time `json:"built_at"`
	ReceivedAt    time.Time  `gorm:"not null;autoCreateTime" json:"received_at"`
}
