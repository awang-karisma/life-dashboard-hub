package models

import "time"

type Record struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Type          string     `gorm:"not null;index:idx_records_type_ts" json:"type"`
	Ts            time.Time  `gorm:"not null;index:idx_records_type_ts" json:"ts"`
	UUID          string     `gorm:"uniqueIndex;not null" json:"uuid"`
	SourceApp     string     `json:"source_app"`
	PayloadSource string     `json:"payload_source"`
	ReceivedAt    time.Time  `gorm:"not null;autoCreateTime" json:"received_at"`
	Data          string     `gorm:"type:text;not null" json:"data"` // JSON text
	DeviceID      uint       `gorm:"not null;index:idx_records_device_type_day" json:"device_id"`
	Seq           *int64     `json:"seq"`
	BuiltAt       *time.Time `json:"built_at"`
	Day           *string    `gorm:"index:idx_records_device_type_day" json:"day"` // "YYYY-MM-DD" string
	DeviceModel   string     `json:"device_model"`
	StartTs       *time.Time `json:"start_ts"`
	RecordID      string     `gorm:"index" json:"record_id"` // Split UUID on '#', first part
}
