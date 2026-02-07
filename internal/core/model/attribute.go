package model

import "github.com/google/uuid"

// Attribute represents the Attribute table in the database.
type Attribute struct {
	ID   uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name string    `gorm:"size:100;not null" json:"name"`
}



// AttributeValue represents the AttributeValue table in the database.
type AttributeValue struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AttID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"att_id"`
	Attribute *Attribute `gorm:"foreignKey:AttID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"attribute"`
	Value     string     `gorm:"size:100;not null" json:"value"`
}
