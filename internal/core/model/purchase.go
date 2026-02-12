package model

import (
	"time"

	"github.com/google/uuid"
)

// Supplier represents the Supplier table.
type Supplier struct {
	ID      uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name    string    `gorm:"size:200;not null" json:"name"`
	Phone   string    `gorm:"size:30" json:"phone"`
	Email   string    `gorm:"size:150" json:"email"`
	Address string    `gorm:"type:text" json:"address"`

	GSTNumber string `gorm:"size:15;uniqueIndex" json:"gst_number"` // ✅ Added

	PurchaseOrders []PurchaseOrder `gorm:"foreignKey:SupplierID" json:"purchase_orders"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

// PurchaseOrder represents the PurchaseOrder table.
type PurchaseOrder struct {
	ID           uuid.UUID           `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PONumber     string              `gorm:"size:50;not null" json:"po_number"`
	SupplierID   uuid.UUID           `gorm:"type:uuid;not null;index" json:"supplier_id"`
	Supplier     Supplier            `gorm:"foreignKey:SupplierID;references:ID" json:"supplier"`
	WarehouseID  uuid.UUID           `gorm:"type:uuid;not null;index" json:"warehouse_id"`
	Warehouse    Warehouse           `gorm:"foreignKey:WarehouseID;references:ID" json:"warehouse"`
	Status       string              `gorm:"size:30" json:"status"`
	OrderDate    time.Time           `json:"order_date"`
	ExpectedDate time.Time           `json:"expected_date"`
	CreatedAt    time.Time           `gorm:"autoCreateTime" json:"created_at"`
	Items        []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID" json:"items"`
}

// PurchaseOrderItem represents the PurchaseOrderItem table.
type PurchaseOrderItem struct {
	ID              uuid.UUID     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PurchaseOrderID uuid.UUID     `gorm:"type:uuid;not null;index" json:"purchase_order_id"`
	PurchaseOrder   PurchaseOrder `gorm:"foreignKey:PurchaseOrderID;references:ID" json:"purchase_order"`
	VariantID       uuid.UUID     `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant         Variant       `gorm:"foreignKey:VariantID;references:ID" json:"variant"`
	Quantity        int           `gorm:"not null" json:"quantity"`
}
