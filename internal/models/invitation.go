package models

import "time"

type Invitation struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	FamilyID     uint       `gorm:"not null;index" json:"family_id"`
	Family       Family     `gorm:"foreignKey:FamilyID" json:"family,omitempty"`
	TokenHash    string     `gorm:"uniqueIndex;not null" json:"-"`
	InvitedEmail string     `json:"invited_email"`
	Role         string     `gorm:"not null" json:"role"`                      // parent, adult, child
	Status       string     `gorm:"not null;default:'pending'" json:"status"` // pending, accepted, expired, revoked
	CreatedBy    uint       `gorm:"not null" json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `gorm:"not null" json:"expires_at"`
	AcceptedAt   *time.Time `json:"accepted_at"`
}
