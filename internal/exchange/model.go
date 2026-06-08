package exchange

import "time"

const (
	StatusCompleted = "COMPLETED"
	StatusCancelled = "CANCELLED"

	DirectionCollect = "COLLECT"
	DirectionRefund  = "REFUND"
)

type ExchangeOrder struct {
	ID                     string     `json:"id"`
	ExchangeNumber         string     `json:"exchange_number"`
	OriginalSalesInvoiceID string     `json:"original_sales_invoice_id"`
	CreditNoteID           *string    `json:"credit_note_id"`
	NewSalesInvoiceID      *string    `json:"new_sales_invoice_id"`
	BranchID               *string    `json:"branch_id"`
	WarehouseID            string     `json:"warehouse_id"`
	CustomerID             string     `json:"customer_id"`
	Status                 string     `json:"status"`
	ItemsOutTotal          float64    `json:"items_out_total"`
	ItemsInTotal           float64    `json:"items_in_total"`
	NetAmount              float64    `json:"net_amount"`
	Notes                  string     `json:"notes"`
	CreatedBy              string     `json:"created_by"`
	CreatedAt              time.Time  `json:"created_at"`
	CompletedAt            *time.Time `json:"completed_at"`
}

type ExchangeItemOut struct {
	ID                 string  `json:"id"`
	ExchangeOrderID    string  `json:"exchange_order_id"`
	SalesInvoiceItemID string  `json:"sales_invoice_item_id"`
	VariantID          string  `json:"variant_id"`
	Quantity           float64 `json:"quantity"`
	UnitPrice          float64 `json:"unit_price"`
	Discount           float64 `json:"discount"`
	BillDiscountShare  float64 `json:"bill_discount_share"`
	TaxPercent         float64 `json:"tax_percent"`
	TaxAmount          float64 `json:"tax_amount"`
	TotalPrice         float64 `json:"total_price"`
	Reason             string  `json:"reason"`
}

type ExchangeItemIn struct {
	ID              string  `json:"id"`
	ExchangeOrderID string  `json:"exchange_order_id"`
	VariantID       string  `json:"variant_id"`
	Quantity        float64 `json:"quantity"`
	UnitPrice       float64 `json:"unit_price"`
	Discount        float64 `json:"discount"`
	TaxPercent      float64 `json:"tax_percent"`
	TaxAmount       float64 `json:"tax_amount"`
	TotalPrice      float64 `json:"total_price"`
}

type ExchangeSettlement struct {
	ID              string    `json:"id"`
	ExchangeOrderID string    `json:"exchange_order_id"`
	Amount          float64   `json:"amount"`
	PaymentMethod   string    `json:"payment_method"`
	Direction       string    `json:"direction"`
	Reference       string    `json:"reference"`
	SettledAt       time.Time `json:"settled_at"`
}
