package purchasereturn

// CreateReturnItem is one item line in the return form.
type CreateReturnItem struct {
	PurchaseOrderItemID string  `json:"purchase_order_item_id"` // optional link
	ItemName            string  `json:"item_name"`
	HSNCode             string  `json:"hsn_code"`
	Unit                string  `json:"unit"`
	Quantity            float64 `json:"quantity"`
	UnitPrice           float64 `json:"unit_price"`
	GSTPercent          float64 `json:"gst_percent"`
	TaxInclusive        bool    `json:"tax_inclusive"`
	Reason              string  `json:"reason"` // per-item reason
}

// CreatePurchaseReturnInput is the payload for POST /purchase-returns.
type CreatePurchaseReturnInput struct {
	PRDate        string             `json:"pr_date"`        // YYYY-MM-DD
	SupplierID    string             `json:"supplier_id"`    // required
	InvoiceNumber string             `json:"invoice_number"` // Bill# — maps to purchase_invoice_id; GRN auto-resolved
	Currency      string             `json:"currency"`       // default Rs
	ExchangeRate  float64            `json:"exchange_rate"`  // default 1
	DutyAmount    float64            `json:"duty_amount"`
	RoundOff      float64            `json:"round_off"`
	Reason        string             `json:"reason"` // overall reason (footer textarea)
	Items         []CreateReturnItem `json:"items"`
}

// PurchaseReturnListRow is one row in the list view.
type PurchaseReturnListRow struct {
	ID                string  `json:"id"`
	PRNumber          string  `json:"pr_number"`
	PRDate            string  `json:"pr_date"`
	SupplierID        string  `json:"supplier_id"`
	SupplierName      string  `json:"supplier_name"`
	GRNNumber         string  `json:"grn_number"`
	GoodsReceiptID    string  `json:"goods_receipt_id"`
	PurchaseInvoiceID string  `json:"purchase_invoice_id"`
	InvoiceNumber     string  `json:"invoice_number"`
	SubAmount         float64 `json:"sub_amount"`
	TaxAmount         float64 `json:"tax_amount"`
	NetAmount         float64 `json:"net_amount"`
	Status            string  `json:"status"`
}

// PurchaseReturnDetailItem is one item in the detail view.
type PurchaseReturnDetailItem struct {
	ID                  string  `json:"id"`
	PurchaseOrderItemID string  `json:"purchase_order_item_id"`
	ItemName            string  `json:"item_name"`
	HSNCode             string  `json:"hsn_code"`
	Unit                string  `json:"unit"`
	Quantity            float64 `json:"quantity"`
	UnitPrice           float64 `json:"unit_price"`
	GSTPercent          float64 `json:"gst_percent"`
	GSTAmount           float64 `json:"gst_amount"`
	TotalAmount         float64 `json:"total_amount"`
	Reason              string  `json:"reason"`
	TaxInclusive        bool    `json:"tax_inclusive"`
}

// PurchaseReturnDetail is the full detail for GET /purchase-returns/:id.
type PurchaseReturnDetail struct {
	ID                string                     `json:"id"`
	PRNumber          string                     `json:"pr_number"`
	PRDate            string                     `json:"pr_date"`
	SupplierID        string                     `json:"supplier_id"`
	SupplierName      string                     `json:"supplier_name"`
	PurchaseInvoiceID string                     `json:"purchase_invoice_id"`
	InvoiceNumber     string                     `json:"invoice_number"`
	GoodsReceiptID    string                     `json:"goods_receipt_id"`
	GRNNumber         string                     `json:"grn_number"`
	Currency          string                     `json:"currency"`
	ExchangeRate      float64                    `json:"exchange_rate"`
	SubAmount         float64                    `json:"sub_amount"`
	TaxAmount         float64                    `json:"tax_amount"`
	DutyAmount        float64                    `json:"duty_amount"`
	RoundOff          float64                    `json:"round_off"`
	NetAmount         float64                    `json:"net_amount"`
	Reason            string                     `json:"reason"`
	Status            string                     `json:"status"`
	CreatedAt         string                     `json:"created_at"`
	Items             []PurchaseReturnDetailItem `json:"items"`
}

// ListFilter holds optional query filters.
type ListFilter struct {
	SupplierName string // partial, case-insensitive
	PRNumber     string // partial, case-insensitive
	GRNNumber    string // partial, case-insensitive
	DateFrom     string // YYYY-MM-DD
	DateTo       string // YYYY-MM-DD
}

// InvoiceLookupItem is one pre-populated item line returned by the invoice lookup.
type InvoiceLookupItem struct {
	PurchaseOrderItemID string  `json:"purchase_order_item_id"`
	ItemName            string  `json:"item_name"`
	HSNCode             string  `json:"hsn_code"`
	Unit                string  `json:"unit"`
	Quantity            float64 `json:"quantity"` // invoiced qty — useful as a max reference
	UnitPrice           float64 `json:"unit_price"`
	GSTPercent          float64 `json:"gst_percent"`
}

// InvoiceLookupResponse is the response for GET /purchase-returns/invoice-lookup?invoice_number=X
type InvoiceLookupResponse struct {
	InvoiceNumber string              `json:"invoice_number"`
	SupplierID    string              `json:"supplier_id"`
	SupplierName  string              `json:"supplier_name"`
	Currency      string              `json:"currency"`      // defaults: Rs
	ExchangeRate  float64             `json:"exchange_rate"` // defaults: 1
	Items         []InvoiceLookupItem `json:"items"`
}
