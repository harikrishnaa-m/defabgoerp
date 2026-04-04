package billing

type BillItemInput struct {
	VariantID    string  `json:"variant_id"`
	ItemType     string  `json:"item_type"` // "PRODUCT" or "MATERIAL" (defaults to "PRODUCT")
	Quantity     float64 `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
	Discount     float64 `json:"discount"`      // discount value (flat amount or percentage based on discount_type)
	DiscountType string  `json:"discount_type"` // "flat" or "percent" (defaults to "flat")
	TaxPercent   float64 `json:"tax_percent"`   // auto-calculated; caller value is ignored
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
	WarehouseID   string `json:"-"`              // auto-resolved from user's branch

	// Items
	Items []BillItemInput `json:"items"`

	// Payments (can be split)
	Payments []PaymentInput `json:"payments"`

	BillDiscount     float64 `json:"bill_discount"`      // discount value on total bill (before tax)
	BillDiscountType string  `json:"bill_discount_type"` // "flat" or "percent" (defaults to "flat")

	Notes string `json:"notes"`
}
