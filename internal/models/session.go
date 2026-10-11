package models

import "time"

type Session struct {
	ID        string     `gorm:"primaryKey" json:"id"` // SHA-256 hash of token
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	User      User       `gorm:"foreignKey:UserID" json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}
