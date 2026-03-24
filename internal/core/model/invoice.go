package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

//////////////////////////////////////////////////////////////
// SALES INVOICE
//////////////////////////////////////////////////////////////

type SalesInvoice struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	// One invoice per sales order
	SalesOrderID uuid.UUID   `gorm:"type:uuid;not null;uniqueIndex" json:"sales_order_id"`
	SalesOrder   *SalesOrder `gorm:"foreignKey:SalesOrderID;references:ID" json:"sales_order,omitempty"`

	CustomerID uuid.UUID `gorm:"type:uuid;not null;index" json:"customer_id"`
	Customer   *Customer `gorm:"foreignKey:CustomerID;references:ID" json:"customer,omitempty"`

	WarehouseID uuid.UUID  `gorm:"type:uuid;not null;index" json:"warehouse_id"`
	Warehouse   *Warehouse `gorm:"foreignKey:WarehouseID;references:ID" json:"warehouse,omitempty"`

	Channel  string     `gorm:"size:20;not null;default:'STORE'" json:"channel"` // STORE or ONLINE
	BranchID *uuid.UUID `gorm:"type:uuid;index" json:"branch_id"`                // null for online orders
	Branch   *Branch    `gorm:"foreignKey:BranchID;references:ID" json:"branch,omitempty"`

	InvoiceNumber string    `gorm:"size:50;uniqueIndex;not null" json:"invoice_number"`
	InvoiceDate   time.Time `gorm:"not null;index" json:"invoice_date"`

	SubAmount      decimal.Decimal `gorm:"type:decimal(12,2);default:0" json:"sub_amount"`
	DiscountAmount decimal.Decimal `gorm:"type:decimal(12,2);default:0" json:"discount_amount"`
	GSTAmount      decimal.Decimal `gorm:"type:decimal(12,2);default:0" json:"gst_amount"`
	RoundOff       decimal.Decimal `gorm:"type:decimal(12,2);default:0" json:"round_off"`

	NetAmount  decimal.Decimal `gorm:"type:decimal(12,2);not null" json:"net_amount"`
	PaidAmount decimal.Decimal `gorm:"type:decimal(12,2);default:0" json:"paid_amount"`

	Status    string    `gorm:"size:20;not null;index" json:"status"` // PAID, PARTIAL, UNPAID, CANCELLED
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Payments []SalesPayment     `gorm:"foreignKey:SalesInvoiceID;constraint:OnDelete:CASCADE" json:"payments,omitempty"`
	Items    []SalesInvoiceItem `gorm:"foreignKey:SalesInvoiceID;constraint:OnDelete:CASCADE" json:"items,omitempty"`
}

//////////////////////////////////////////////////////////////
// SALES PAYMENT
//////////////////////////////////////////////////////////////

type SalesPayment struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	SalesInvoiceID uuid.UUID     `gorm:"type:uuid;not null;index" json:"sales_invoice_id"`
	SalesInvoice   *SalesInvoice `gorm:"foreignKey:SalesInvoiceID;references:ID;constraint:OnDelete:CASCADE" json:"sales_invoice,omitempty"`

	Amount decimal.Decimal `gorm:"type:decimal(12,2);not null" json:"amount"`

	PaymentMethod string `gorm:"size:20;not null;index" json:"payment_method"` // CASH, UPI, CARD, BANK_TRANSFER
	Reference     string `gorm:"size:50" json:"reference"`

	PaidAt time.Time `gorm:"index" json:"paid_at"`
}

//////////////////////////////////////////////////////////////
// SALES INVOICE ITEM
//////////////////////////////////////////////////////////////

type SalesInvoiceItem struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	SalesInvoiceID uuid.UUID     `gorm:"type:uuid;not null;index" json:"sales_invoice_id"`
	SalesInvoice   *SalesInvoice `gorm:"foreignKey:SalesInvoiceID;references:ID;constraint:OnDelete:CASCADE" json:"sales_invoice,omitempty"`

	VariantID uuid.UUID `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant   *Variant  `gorm:"foreignKey:VariantID;references:ID" json:"variant,omitempty"`

	Quantity   int             `gorm:"not null" json:"quantity"`
	UnitPrice  decimal.Decimal `gorm:"type:decimal(12,2);not null" json:"unit_price"`
	Discount   decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"discount"`
	TaxPercent decimal.Decimal `gorm:"type:decimal(5,2);not null;default:0" json:"tax_percent"`
	TaxAmount  decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"tax_amount"`
	TotalPrice decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"total_price"`
}

//////////////////////////////////////////////////////////////
// PURCHASE INVOICE
//////////////////////////////////////////////////////////////

type PurchaseInvoice struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	PurchaseOrderID uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex" json:"purchase_order_id"`
	PurchaseOrder   *PurchaseOrder `gorm:"foreignKey:PurchaseOrderID;references:ID" json:"purchase_order,omitempty"`

	SupplierID uuid.UUID `gorm:"type:uuid;not null;index" json:"supplier_id"`
	Supplier   *Supplier `gorm:"foreignKey:SupplierID;references:ID" json:"supplier,omitempty"`

	WarehouseID uuid.UUID  `gorm:"type:uuid;not null;index" json:"warehouse_id"`
	Warehouse   *Warehouse `gorm:"foreignKey:WarehouseID;references:ID" json:"warehouse,omitempty"`

	InvoiceNumber string    `gorm:"size:50;uniqueIndex;not null" json:"invoice_number"`
	InvoiceDate   time.Time `gorm:"not null;index" json:"invoice_date"`

	SubAmount      float64 `gorm:"type:numeric(12,2);default:0" json:"sub_amount"`
	DiscountAmount float64 `gorm:"type:numeric(12,2);default:0" json:"discount_amount"`
	GSTAmount      float64 `gorm:"type:numeric(12,2);default:0" json:"gst_amount"`
	RoundOff       float64 `gorm:"type:numeric(12,2);default:0" json:"round_off"`

	NetAmount  float64 `gorm:"type:numeric(12,2);not null" json:"net_amount"`
	PaidAmount float64 `gorm:"type:numeric(12,2);default:0" json:"paid_amount"`

	Status string `gorm:"size:20;not null;index" json:"status"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	Payments []SupplierPayment     `gorm:"foreignKey:PurchaseInvoiceID;constraint:OnDelete:CASCADE" json:"payments,omitempty"`
	Items    []PurchaseInvoiceItem `gorm:"foreignKey:PurchaseInvoiceID;constraint:OnDelete:CASCADE" json:"items,omitempty"`
}

//////////////////////////////////////////////////////////////
// SUPPLIER PAYMENT
//////////////////////////////////////////////////////////////

type SupplierPayment struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	PurchaseInvoiceID uuid.UUID        `gorm:"type:uuid;not null;index" json:"purchase_invoice_id"`
	PurchaseInvoice   *PurchaseInvoice `gorm:"foreignKey:PurchaseInvoiceID;references:ID;constraint:OnDelete:CASCADE" json:"purchase_invoice,omitempty"`

	Amount float64 `gorm:"type:numeric(12,2);not null" json:"amount"`

	PaymentMethod string `gorm:"size:20;not null;index" json:"payment_method"` // CASH, UPI, CARD, BANK_TRANSFER
	Reference     string `gorm:"size:50" json:"reference"`

	PaidAt time.Time `gorm:"index" json:"paid_at"`
}

//////////////////////////////////////////////////////////////
// PURCHASE INVOICE ITEM
//////////////////////////////////////////////////////////////

type PurchaseInvoiceItem struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`

	PurchaseInvoiceID uuid.UUID        `gorm:"type:uuid;not null;index" json:"purchase_invoice_id"`
	PurchaseInvoice   *PurchaseInvoice `gorm:"foreignKey:PurchaseInvoiceID;references:ID;constraint:OnDelete:CASCADE" json:"purchase_invoice,omitempty"`

	VariantID uuid.UUID `gorm:"type:uuid;not null;index" json:"variant_id"`
	Variant   *Variant  `gorm:"foreignKey:VariantID;references:ID" json:"variant,omitempty"`

	Quantity float64 `gorm:"type:numeric(10,3);not null" json:"quantity"`

	UnitCost    float64 `gorm:"type:numeric(12,2);not null" json:"unit_cost"`
	TaxAmount   float64 `gorm:"type:numeric(12,2);default:0" json:"tax_amount"`
	TotalAmount float64 `gorm:"type:numeric(12,2);not null" json:"total_amount"`
}
