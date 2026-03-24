package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Supplier represents the Supplier table.
type Supplier struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SupplierCode string    `gorm:"size:50;uniqueIndex" json:"supplier_code"` // e.g. SUP-001
	Name         string    `gorm:"size:200;not null" json:"name"`
	Phone        string    `gorm:"size:30" json:"phone"`
	Email        string    `gorm:"size:150" json:"email"`
	Address      string    `gorm:"type:text" json:"address"`

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
	TotalAmount  decimal.Decimal     `gorm:"type:decimal(12,2);not null;default:0" json:"total_amount"`
	TaxAmount    decimal.Decimal     `gorm:"type:decimal(10,2);not null;default:0" json:"tax_amount"`
	GrandTotal   decimal.Decimal     `gorm:"type:decimal(12,2);not null;default:0" json:"grand_total"`
	CreatedAt    time.Time           `gorm:"autoCreateTime" json:"created_at"`
	Items        []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID" json:"items"`
}

// PurchaseOrderItem represents a line item (raw material) in a purchase order.
type PurchaseOrderItem struct {
	ID              uuid.UUID     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	PurchaseOrderID uuid.UUID     `gorm:"type:uuid;not null;index" json:"purchase_order_id"`
	PurchaseOrder   PurchaseOrder `gorm:"foreignKey:PurchaseOrderID;references:ID" json:"purchase_order"`

	ItemName    string `gorm:"size:200;not null" json:"item_name"` // e.g. "Cotton Fabric 60 inch"
	Description string `gorm:"type:text" json:"description"`       // optional details
	HSNCode     string `gorm:"size:20" json:"hsn_code"`            // GST HSN code
	Unit        string `gorm:"size:20;not null" json:"unit"`       // METER, KG, PCS, ROLL, etc.

	Quantity    decimal.Decimal `gorm:"type:decimal(10,2);not null" json:"quantity"`
	UnitPrice   decimal.Decimal `gorm:"type:decimal(10,2);not null" json:"unit_price"`
	GSTPercent  decimal.Decimal `gorm:"type:decimal(5,2);not null;default:0" json:"gst_percent"`
	GSTAmount   decimal.Decimal `gorm:"type:decimal(10,2);not null;default:0" json:"gst_amount"`
	TotalPrice  decimal.Decimal `gorm:"type:decimal(12,2);not null" json:"total_price"`
	ReceivedQty decimal.Decimal `gorm:"type:decimal(10,2);default:0" json:"received_qty"`
}
