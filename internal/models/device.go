package models

import "time"

type Device struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	UserID           uint       `gorm:"not null;index;uniqueIndex:idx_user_label" json:"user_id"`
	User             User       `gorm:"foreignKey:UserID" json:"-"`
	Label            string     `gorm:"not null;uniqueIndex:idx_user_label" json:"label"`
	TokenHash        string     `gorm:"uniqueIndex;not null" json:"-"`
	DisplayName      string     `gorm:"not null;default:''" json:"display_name"`
	WebhookSecret    string     `gorm:"not null;default:''" json:"-"` // plaintext HMAC secret
	DeviceModel      string     `gorm:"not null;default:''" json:"device_model"`
	CollectorVersion string     `gorm:"not null;default:''" json:"collector_version"`
	Status           string     `gorm:"not null;default:'active'" json:"status"` // active, revoked
	LastSyncAt       *time.Time `json:"last_sync_at"`
	LastSyncSource   string     `json:"last_sync_source"`
	LastSyncError    string     `json:"last_sync_error"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at"`
}
