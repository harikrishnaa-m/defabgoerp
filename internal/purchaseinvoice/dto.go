package purchaseinvoice

type CreatePurchaseInvoiceInput struct {
	PurchaseOrderID string  `json:"purchase_order_id"`
	InvoiceNumber   string  `json:"invoice_number"`
	InvoiceDate     string  `json:"invoice_date"` // YYYY-MM-DD
	DiscountAmount  float64 `json:"discount_amount"`
	RoundOff        float64 `json:"round_off"`
	Notes           string  `json:"notes"`

	// Optional payment at creation time
	PaymentMethod string  `json:"payment_method"` // CASH, UPI, CARD, BANK_TRANSFER (optional)
	PaymentAmount float64 `json:"payment_amount"` // if 0, no payment recorded
	Reference     string  `json:"reference"`      // transaction ID etc. Not needed for CASH
}

type RecordPaymentInput struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"` // CASH, UPI, CARD, BANK_TRANSFER
	Reference     string  `json:"reference"`
	PaidAt        string  `json:"paid_at"` // optional, defaults to NOW
}
