package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ================== CUSTOMER ==================

type Customer struct {
	ID             uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CustomerCode   string          `gorm:"size:50;uniqueIndex" json:"customer_code"` // e.g. CUS001
	Name           string          `gorm:"size:200;not null" json:"name"`
	Phone          string          `gorm:"size:30" json:"phone"`
	Email          string          `gorm:"size:150" json:"email"`
	TotalPurchases decimal.Decimal `gorm:"type:decimal(14,2);not null;default:0" json:"total_purchases"`
	IsActive       bool            `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`

	SalesOrders []SalesOrder `gorm:"foreignKey:CustomerID" json:"sales_orders"`
}

// ================== SALES PERSON ==================

type SalesPerson struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID       *uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	User         *User      `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
	BranchID     *uuid.UUID `gorm:"type:uuid;index" json:"branch_id"`
	Branch       *Branch    `gorm:"foreignKey:BranchID;references:ID" json:"branch,omitempty"`
	Name         string     `gorm:"size:200;not null" json:"name"`
	EmployeeCode string     `gorm:"size:50;uniqueIndex" json:"employee_code"`
	Phone        string     `gorm:"size:30" json:"phone"`
	Email        string     `gorm:"size:150" json:"email"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`

	SalesOrders []SalesOrder `gorm:"foreignKey:SalesPersonID" json:"sales_orders"`
}

// ================== SALES ORDER ==================
// Channel: STORE = in-store/POS sale, ONLINE = web/e-commerce sale

type SalesOrder struct {
	ID            uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SONumber      string          `gorm:"size:50;not null;uniqueIndex" json:"so_number"`
	Channel       string          `gorm:"size:20;not null;default:'STORE'" json:"channel"` // STORE or ONLINE
	BranchID      *uuid.UUID      `gorm:"type:uuid;index" json:"branch_id"`                // null for online orders
	Branch        *Branch         `gorm:"foreignKey:BranchID;references:ID" json:"branch,omitempty"`
	CustomerID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"customer_id"`
	Customer      Customer        `gorm:"foreignKey:CustomerID;references:ID" json:"customer"`
	SalesPersonID *uuid.UUID      `gorm:"type:uuid;index" json:"salesperson_id"` // nullable for online
	SalesPerson   *SalesPerson    `gorm:"foreignKey:SalesPersonID;references:ID" json:"salesperson,omitempty"`
	WarehouseID   uuid.UUID       `gorm:"type:uuid;not null;index" json:"warehouse_id"`
	Warehouse     Warehouse       `gorm:"foreignKey:WarehouseID;references:ID" json:"warehouse"`
	CreatedBy     uuid.UUID       `gorm:"type:uuid;not null" json:"created_by"`
	OrderDate     time.Time       `gorm:"not null" json:"order_date"`
	Subtotal      decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"subtotal"`
	TaxTotal      decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"tax_total"`
	DiscountTotal decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"discount_total"`
	GrandTotal    decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"grand_total"`
	Status        string          `gorm:"size:30;not null;default:'DRAFT'" json:"status"`          // DRAFT, CONFIRMED, DISPATCHED, DELIVERED, CANCELLED, RETURNED
	PaymentStatus string          `gorm:"size:20;not null;default:'UNPAID'" json:"payment_status"` // UNPAID, PARTIAL, PAID
	Notes         string          `gorm:"type:text" json:"notes"`
	CreatedAt     time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`

	Items []SalesOrderItem `gorm:"foreignKey:SalesOrderID" json:"items"`
}

// ================== SALES ORDER ITEM ==================

type SalesOrderItem struct {
	ID           uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	SalesOrderID uuid.UUID       `gorm:"type:uuid;not null;index" json:"sales_order_id"`
	SalesOrder   SalesOrder      `gorm:"foreignKey:SalesOrderID;references:ID" json:"sales_order"`
	VariantID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant      Variant         `gorm:"foreignKey:VariantID;references:ID" json:"variant"`
	Quantity     int             `gorm:"not null" json:"quantity"`
	UnitPrice    decimal.Decimal `gorm:"type:decimal(12,2);not null" json:"unit_price"`
	Discount     decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"discount"`
	TaxPercent   decimal.Decimal `gorm:"type:decimal(5,2);not null;default:0" json:"tax_percent"`
	TaxAmount    decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"tax_amount"`
	TotalPrice   decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"total_price"`
}
