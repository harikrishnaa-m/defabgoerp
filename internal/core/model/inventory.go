package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Branch struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Address     string    `json:"address"`
	ManagerID   uuid.UUID `json:"manager_id"`
	BranchCode  string    `gorm:"size:50" json:"branch_code"`
	City        string    `gorm:"size:100" json:"city"`
	State       string    `gorm:"size:100" json:"state"`
	PhoneNumber string    `gorm:"size:20" json:"phone_number"`
}

type Warehouse struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BranchID      *uint     `json:"branch_id"`
	Branch        *Branch   `gorm:"foreignKey:BranchID;references:ID" json:"branch"`
	Name          string    `gorm:"size:150;not null" json:"name"`
	Type          string    `gorm:"default:'STORE'" json:"type"` // "CENTRAL", "STORE", "FACTORY"
	WarehouseCode string    `gorm:"size:50" json:"warehouse_code"`
}

type Stock struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	VariantID   uuid.UUID `gorm:"type:uuid;not null;index:idx_variant_warehouse,unique"`
	WarehouseID uuid.UUID `gorm:"type:uuid;not null;index:idx_variant_warehouse,unique"`

	Variant   Variant   `gorm:"foreignKey:VariantID;constraint:OnDelete:CASCADE"`
	Warehouse Warehouse `gorm:"foreignKey:WarehouseID;constraint:OnDelete:CASCADE"`

	Quantity  decimal.Decimal `gorm:"type:decimal(10,2);not null;default:0"`
	StockType string          `gorm:"size:20;not null;default:'PRODUCT'"` // RAW_MATERIAL or PRODUCT

	UpdatedAt time.Time
}

type StockMovement struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	VariantID uuid.UUID `gorm:"type:uuid;not null;index"`
	Variant   Variant   `gorm:"foreignKey:VariantID;constraint:OnDelete:CASCADE"`

	FromWarehouseID *uuid.UUID `gorm:"type:uuid;index"`
	FromWarehouse   *Warehouse `gorm:"foreignKey:FromWarehouseID"`

	ToWarehouseID *uuid.UUID `gorm:"type:uuid;index"`
	ToWarehouse   *Warehouse `gorm:"foreignKey:ToWarehouseID"`

	Quantity decimal.Decimal `gorm:"type:decimal(10,2);not null"`

	MovementType string `gorm:"size:20;not null"`
	// IN, OUT, TRANSFER

	StockRequestID *uuid.UUID `gorm:"type:uuid;index"`

	Status string `gorm:"size:20;default:'COMPLETED'"`
	// PENDING, IN_TRANSIT, RECEIVED, CANCELLED, COMPLETED

	//	Reference string `gorm:"size:100"`
	PurchaseOrderID *uuid.UUID `gorm:"type:uuid;index"`
	SaleOrderID     *uuid.UUID `gorm:"type:uuid;index"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time

	SupplierID *uuid.UUID `gorm:"type:uuid;index"` // ✅ NEW

	Reference string `gorm:"size:100"`
}

// GoodsReceipt represents a goods receipt note (GRN) linked to a purchase order.
type GoodsReceipt struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	GRNNumber string `gorm:"size:50;uniqueIndex;not null" json:"grn_number"` // e.g. GRN-001

	PurchaseOrderID uuid.UUID      `gorm:"type:uuid;not null;index" json:"purchase_order_id"`
	PurchaseOrder   *PurchaseOrder `gorm:"foreignKey:PurchaseOrderID;references:ID" json:"purchase_order,omitempty"`

	SupplierID uuid.UUID `gorm:"type:uuid;not null;index" json:"supplier_id"`
	Supplier   *Supplier `gorm:"foreignKey:SupplierID;references:ID" json:"supplier,omitempty"`

	WarehouseID uuid.UUID  `gorm:"type:uuid;not null;index" json:"warehouse_id"`
	Warehouse   *Warehouse `gorm:"foreignKey:WarehouseID;references:ID" json:"warehouse,omitempty"`

	ReceivedBy uuid.UUID `gorm:"type:uuid;not null" json:"received_by"`

	ReceivedDate time.Time `gorm:"not null" json:"received_date"`
	Reference    string    `gorm:"size:100" json:"reference"` // Invoice / DC number

	Status string `gorm:"size:20;not null;default:'COMPLETED'" json:"status"` // COMPLETED, CANCELLED

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	Items []GoodsReceiptItem `gorm:"foreignKey:GoodsReceiptID;constraint:OnDelete:CASCADE" json:"items,omitempty"`
}

// GoodsReceiptItem represents a line item in a goods receipt.
type GoodsReceiptItem struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	GoodsReceiptID uuid.UUID     `gorm:"type:uuid;not null;index" json:"goods_receipt_id"`
	GoodsReceipt   *GoodsReceipt `gorm:"foreignKey:GoodsReceiptID;references:ID;constraint:OnDelete:CASCADE" json:"goods_receipt,omitempty"`

	PurchaseOrderItemID uuid.UUID          `gorm:"type:uuid;not null;index" json:"purchase_order_item_id"`
	PurchaseOrderItem   *PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderItemID;references:ID" json:"purchase_order_item,omitempty"`

	OrderedQty  decimal.Decimal `gorm:"type:decimal(10,2);not null" json:"ordered_qty"`
	ReceivedQty decimal.Decimal `gorm:"type:decimal(10,2);not null" json:"received_qty"`
}

type StockRequest struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	FromWarehouseID uuid.UUID `gorm:"type:uuid;not null;index"`
	FromWarehouse   Warehouse `gorm:"foreignKey:FromWarehouseID"`

	ToWarehouseID uuid.UUID `gorm:"type:uuid;not null;index"`
	ToWarehouse   Warehouse `gorm:"foreignKey:ToWarehouseID"`

	Status string `gorm:"size:20;not null;default:'PENDING'"`
	// PENDING, APPROVED, PARTIAL, REJECTED, CANCELLED

	Priority string `gorm:"size:10;default:'MEDIUM'"`
	// LOW, MEDIUM, HIGH

	ExpectedDate *time.Time `json:"expected_date"` // ✅ Added

	RequestedBy uuid.UUID `gorm:"type:uuid;not null"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Items     []StockRequestItem     `gorm:"foreignKey:StockRequestID"`
	Approvals []StockRequestApproval `gorm:"foreignKey:StockRequestID"`
}

type StockRequestItem struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	StockRequestID uuid.UUID `gorm:"type:uuid;not null;index"`
	StockRequest   StockRequest

	VariantID uuid.UUID `gorm:"type:uuid;not null"`
	Variant   Variant   `gorm:"foreignKey:VariantID"`

	//RequestedQty int `gorm:"not null"`

	//ApprovedQty int `gorm:"default:0"`

	RequestedQty decimal.Decimal `gorm:"type:decimal(10,2);not null"`

	ApprovedQty decimal.Decimal `gorm:"type:decimal(10,2);default:0"`

	Remarks string `gorm:"type:text"`
}

type StockRequestApproval struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	StockRequestID uuid.UUID    `gorm:"type:uuid;not null;index"`
	StockRequest   StockRequest `gorm:"foreignKey:StockRequestID;constraint:OnDelete:CASCADE"`

	Action string `gorm:"size:20;not null"`
	// SUBMITTED, APPROVED, PARTIAL, REJECTED, CANCELLED

	ApprovedBy uuid.UUID `gorm:"type:uuid;not null"`

	Remarks string `gorm:"type:text"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
}
