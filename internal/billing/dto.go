package billing

type BillItemInput struct {
	VariantID  string  `json:"variant_id"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	Discount   float64 `json:"discount"`
	TaxPercent float64 `json:"tax_percent"`
}

type PaymentInput struct {
	Method    string  `json:"method"` // CASH, UPI, CARD, BANK_TRANSFER
	Amount    float64 `json:"amount"`
	Reference string  `json:"reference"`
}

type CreateBillInput struct {
	// Customer
	CustomerPhone string `json:"customer_phone"`
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`

	// Sale context
	Channel       string `json:"channel"`        // STORE or ONLINE (defaults to STORE)
	SalesPersonID string `json:"salesperson_id"` // optional for ONLINE
	WarehouseID   string `json:"warehouse_id"`

	// Items
	Items []BillItemInput `json:"items"`

	// Payments (can be split)
	Payments []PaymentInput `json:"payments"`

	Notes string `json:"notes"`
}
