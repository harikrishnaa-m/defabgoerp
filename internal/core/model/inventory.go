package model

import (
	"time"

	"github.com/google/uuid"
)

type Branch struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Address   string    `json:"address"`
	ManagerID uuid.UUID `json:"manager_id"`
}

type Warehouse struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BranchID *uint     `json:"branch_id"`
	Branch   *Branch   `gorm:"foreignKey:BranchID;references:ID" json:"branch"`
	Name     string    `gorm:"size:150;not null" json:"name"`
	Type     string    `gorm:"default:'STORE'" json:"type"` // "CENTRAL", "STORE", "FACTORY"
}

type Stock struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	VariantID   uuid.UUID `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant     Variant   `gorm:"foreignKey:VariantID;references:ID;constraint:OnDelete:CASCADE" json:"variant"`
	WarehouseID uuid.UUID `gorm:"type:uuid;not null;index" json:"warehouse_id"`
	Warehouse   Warehouse `gorm:"foreignKey:WarehouseID;references:ID;constraint:OnDelete:CASCADE" json:"warehouse"`
	Quantity    int       `gorm:"not null" json:"quantity"`
}

type StockMovement struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	VariantID       uuid.UUID `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant         Variant   `gorm:"foreignKey:VariantID;references:ID;constraint:OnDelete:CASCADE" json:"variant"`
	FromWarehouseID uuid.UUID `gorm:"type:uuid;index" json:"from_warehouse_id"`
	FromWarehouse   Warehouse `gorm:"foreignKey:FromWarehouseID;references:ID" json:"from_warehouse"`
	ToWarehouseID   uuid.UUID `gorm:"type:uuid;index" json:"to_warehouse_id"`
	ToWarehouse     Warehouse `gorm:"foreignKey:ToWarehouseID;references:ID" json:"to_warehouse"`
	Quantity        int       `gorm:"not null" json:"quantity"`
	MovementType    string    `gorm:"size:20;not null" json:"movement_type"` // IN, OUT, TRANSFER
	Reference       string    `gorm:"size:100" json:"reference"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}
