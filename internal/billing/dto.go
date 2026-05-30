package billing

type BillItemInput struct {
	VariantID    string  `json:"variant_id"`
	ItemType     string  `json:"type"` // "PRODUCT" or "MATERIAL" (defaults to "PRODUCT")
	Quantity     float64 `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
	Discount     float64 `json:"discount"`      // discount value (flat amount or percentage based on discount_type)
	DiscountType string  `json:"discount_type"` // "flat" or "percent" (defaults to "flat")
	TaxPercent   float64 `json:"tax_percent"`   // auto-calculated; caller value is ignored
}

type PaymentInput struct {
	Method    string  `json:"method"` // CASH, UPI, CARD, DEBIT_CARD, CREDIT_CARD, BANK_TRANSFER
	Amount    float64 `json:"amount"`
	Reference string  `json:"reference"`
}

type CreateBillInput struct {
	// Customer
	CustomerPhone      string `json:"customer_phone"`
	CustomerName       string `json:"customer_name"`
	CustomerEmail      string `json:"customer_email"`
	GSTNumber          string `json:"gst_number"`           // optional; saved to customer record
	CustomerBirthDay   *int   `json:"customer_birth_day"`   // 1-31, optional
	CustomerBirthMonth *int   `json:"customer_birth_month"` // 1-12, optional

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

	// Optional: link this bill to a completed return order (for exchange/credit flows).
	// The return_order_id will be stored on both the sales_order and sales_invoice.
	ReturnNumber string `json:"return_number"`

	// Optional: override invoice/order date. Format: "2006-01-02". Defaults to today.
	Date string `json:"date"`
}
