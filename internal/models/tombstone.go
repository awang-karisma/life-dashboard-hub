package models

import "time"

type Tombstone struct {
	Type      string     `gorm:"primaryKey;not null" json:"type"`
	UUID      string     `gorm:"primaryKey;not null" json:"uuid"`
	DeviceID  uint       `gorm:"not null;index" json:"device_id"`
	Seq       *int64     `json:"seq"`
	BuiltAt   *time.Time `json:"built_at"`
	DeletedAt time.Time  `gorm:"not null;autoCreateTime" json:"deleted_at"`
}
