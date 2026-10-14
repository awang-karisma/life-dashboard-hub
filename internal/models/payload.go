package models

import "time"

type Payload struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	DeviceID    uint       `gorm:"not null;index;uniqueIndex:idx_device_body_sha256" json:"device_id"`
	Source      string     `json:"source"` // 'health_connect', 'screen_time'
	Sequence    *int64     `json:"sequence"`
	AppVersion  string     `json:"app_version"`
	BuiltAt     *time.Time `json:"built_at"`
	ReceivedAt  time.Time  `gorm:"not null;autoCreateTime" json:"received_at"`
	Kind        string     `gorm:"not null" json:"kind"` // sync, backfill, deletion, screen_time, ping
	RecordCount int        `gorm:"not null;default:0" json:"record_count"`
	BodySHA256  string     `gorm:"uniqueIndex:idx_device_body_sha256;not null" json:"body_sha256"`
	Body        string     `gorm:"type:text;not null" json:"body"` // JSON text (portable across SQLite/PG)
}
