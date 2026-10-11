package models

import "time"

type FamilyMembership struct {
	ID       uint       `gorm:"primaryKey" json:"id"`
	FamilyID uint       `gorm:"not null;uniqueIndex:idx_family_user" json:"family_id"`
	UserID   uint       `gorm:"not null;uniqueIndex:idx_family_user" json:"user_id"`
	User     User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Family   Family     `gorm:"foreignKey:FamilyID" json:"family,omitempty"`
	Role     string     `gorm:"not null" json:"role"`                     // admin, parent, adult, child
	Sharing  string     `gorm:"not null;default:'none'" json:"sharing"`   // none, summary, detailed
	Status   string     `gorm:"not null;default:'active'" json:"status"` // active, left, removed
	JoinedAt time.Time  `gorm:"not null;autoCreateTime" json:"joined_at"`
	LeftAt   *time.Time `json:"left_at"`
}
