package models

type AppAlias struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserID      uint   `gorm:"not null;uniqueIndex:idx_user_package" json:"user_id"`
	PackageName string `gorm:"not null;uniqueIndex:idx_user_package" json:"package_name"`
	AppName     string `gorm:"not null" json:"app_name"`
}
