package models

import "time"

type AppMetadata struct {
	PackageName string    `gorm:"primaryKey;not null" json:"package_name"`
	AppName     string    `gorm:"not null" json:"app_name"`
	Category    string    `gorm:"not null;default:''" json:"category"` // e.g. PRODUCTIVITY, SOCIAL, GAME_STRATEGY
	Source      string    `gorm:"not null;default:'play_store'" json:"source"` // play_store, custom, fallback
	ResolvedAt  time.Time `gorm:"not null;autoCreateTime" json:"resolved_at"`
}
