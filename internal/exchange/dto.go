package exchange

// ExchangeItemOutInput selects a line from the original invoice to return.
type ExchangeItemOutInput struct {
	SalesInvoiceItemID string  `json:"sales_invoice_item_id"`
	Quantity           float64 `json:"quantity"`
	Reason             string  `json:"reason"`
}

// ExchangeItemInInput describes a new item the customer is taking.
// Set UnitPrice to 0 to auto-fetch from the variants table.
// DiscountType: "flat" (default) or "percent".
// ItemType: "PRODUCT" (default) or "MATERIAL" – used for GST slab resolution.
type ExchangeItemInInput struct {
	VariantID    string  `json:"variant_id"`
	Quantity     float64 `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
	Discount     float64 `json:"discount"`
	DiscountType string  `json:"discount_type"`
	ItemType     string  `json:"item_type"`
}

// ExchangeSettlementInput records how the net cash difference is paid or refunded.
// direction is derived server-side from the sign of net_amount.
type ExchangeSettlementInput struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"` // CASH, CARD, CREDIT_CARD, DEBIT_CARD, UPI, BANK_TRANSFER
	Reference     string  `json:"reference"`
}

// CreateExchangeInput is the single request body for a complete exchange transaction.
// The server creates the credit note and new invoice atomically.
// ExchangeDate is optional (YYYY-MM-DD); defaults to current server time when omitted.
type CreateExchangeInput struct {
	OriginalSalesInvoiceID string                    `json:"original_sales_invoice_id"`
	ItemsOut               []ExchangeItemOutInput    `json:"items_out"`
	ItemsIn                []ExchangeItemInInput     `json:"items_in"`
	Settlements            []ExchangeSettlementInput `json:"settlements"`
	Notes                  string                    `json:"notes"`
	ExchangeDate           string                    `json:"exchange_date"` // YYYY-MM-DD, optional
}

// ExchangeListFilter holds query params for listing exchange orders.
type ExchangeListFilter struct {
	BranchID *string
	Status   string
	Search   string
	Limit    int
	Offset   int
}
