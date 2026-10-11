package models

import "time"

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Email        string     `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string     `gorm:"not null;default:''" json:"-"` // empty for OIDC-only users
	DisplayName  string     `gorm:"not null;default:''" json:"display_name"`
	Timezone     string     `gorm:"not null;default:'UTC'" json:"timezone"`
	Status       string     `gorm:"not null;default:'active'" json:"status"`     // active, suspended, deleted
	AuthMethod   string     `gorm:"not null;default:'local'" json:"auth_method"` // "local" or "oidc"
	OIDCSubject  *string    `gorm:"index:idx_user_oidc" json:"oidc_subject,omitempty"`
	OIDCIssuer   *string    `gorm:"index:idx_user_oidc" json:"oidc_issuer,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
