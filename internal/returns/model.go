package returns

import "time"

type ReturnOrderStatus string

type ReturnOrder struct {
	ID              string    `json:"id"`
	ReturnNumber    string    `json:"return_number"`
	SalesInvoiceID  string    `json:"sales_invoice_id"`
	BranchID        string    `json:"branch_id"`
	WarehouseID     string    `json:"warehouse_id"`
	CustomerID      string    `json:"customer_id"`
	Status          string    `json:"status"`
	RefundType      string    `json:"refund_type"`
	RefundMethod    string    `json:"refund_method"`
	RefundReference string    `json:"refund_reference"`
	RefundAmount    float64   `json:"refund_amount"`
	TotalAmount     float64   `json:"total_amount"`
	GSTAmount       float64   `json:"gst_amount"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	ProcessedAt     time.Time `json:"processed_at"`
	Notes           string    `json:"notes"`
}

type ReturnItem struct {
	ID                 string  `json:"id"`
	ReturnOrderID      string  `json:"return_order_id"`
	SalesInvoiceItemID string  `json:"sales_invoice_item_id"`
	VariantID          string  `json:"variant_id"`
	Quantity           int     `json:"quantity"`
	UnitPrice          float64 `json:"unit_price"`
	Discount           float64 `json:"discount"`
	BillDiscountShare  float64 `json:"bill_discount_share"`
	TaxPercent         float64 `json:"tax_percent"`
	TaxAmount          float64 `json:"tax_amount"`
	TotalPrice         float64 `json:"total_price"`
	Reason             string  `json:"reason"`
}

type ReturnPayment struct {
	ID            string    `json:"id"`
	ReturnOrderID string    `json:"return_order_id"`
	Amount        float64   `json:"amount"`
	PaymentMethod string    `json:"payment_method"`
	Reference     string    `json:"reference"`
	PaidAt        time.Time `json:"paid_at"`
	CreatedAt     time.Time `json:"created_at"`
}

const (
	ReturnStatusRequested = "REQUESTED"
	ReturnStatusCompleted = "COMPLETED"
	ReturnStatusCancelled = "CANCELLED"

	RefundTypeCash   = "CASH"
	RefundTypeCredit = "CREDIT"
)
