package models

type FamilySettings struct {
	ID                   uint   `gorm:"primaryKey" json:"id"`
	FamilyID             uint   `gorm:"uniqueIndex;not null" json:"family_id"`
	Family               Family `gorm:"foreignKey:FamilyID" json:"-"`
	MembersCanViewOthers bool   `gorm:"not null;default:false" json:"members_can_view_others"`
}
