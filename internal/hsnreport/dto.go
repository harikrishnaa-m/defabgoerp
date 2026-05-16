package hsnreport

// HSNSalesRow is one line-item row in the HSN sales report.
type HSNSalesRow struct {
	InvoiceNumber string  `json:"invoice_number"`
	Date          string  `json:"date"` // DD/MM/YYYY
	CustomerName  string  `json:"customer_name"`
	Location      string  `json:"location"`
	VariantName   string  `json:"variant_name"`
	HSNCode       string  `json:"hsn_code"`
	Quantity      float64 `json:"quantity"`
	UnitPrice     float64 `json:"unit_price"`
	Discount      float64 `json:"discount"`
	TaxPercent    float64 `json:"tax_percent"`
	TaxAmount     float64 `json:"tax_amount"`
	TotalPrice    float64 `json:"total_price"`
}

// HSNSalesResult is the full API response for HSN sales.
type HSNSalesResult struct {
	Data          []HSNSalesRow `json:"data"`
	Total         int           `json:"total"`
	TotalQuantity float64       `json:"total_quantity"`
	TotalAmount   float64       `json:"total_amount"`
}

// HSNPurchaseRow is one line-item row in the HSN purchase report.
type HSNPurchaseRow struct {
	PONumber     string  `json:"po_number"`
	Date         string  `json:"date"` // DD/MM/YYYY
	SupplierName string  `json:"supplier_name"`
	ItemName     string  `json:"item_name"`
	HSNCode      string  `json:"hsn_code"`
	Unit         string  `json:"unit"`
	Quantity     float64 `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
	GSTPercent   float64 `json:"gst_percent"`
	GSTAmount    float64 `json:"gst_amount"`
	TotalAmount  float64 `json:"total_amount"`
}

// HSNPurchaseResult is the full API response for HSN purchase.
type HSNPurchaseResult struct {
	Data          []HSNPurchaseRow `json:"data"`
	Total         int              `json:"total"`
	TotalQuantity float64          `json:"total_quantity"`
	TotalAmount   float64          `json:"total_amount"`
}

// HSNJobOrderRow is one material line-item in the HSN job order report.
type HSNJobOrderRow struct {
	JobNumber    string  `json:"job_number"`
	Date         string  `json:"date"` // DD/MM/YYYY (received_date)
	CustomerName string  `json:"customer_name"`
	Location     string  `json:"location"`
	JobType      string  `json:"job_type"`
	Status       string  `json:"status"`
	VariantName  string  `json:"variant_name"`
	HSNCode      string  `json:"hsn_code"`
	QuantityUsed float64 `json:"quantity_used"`
	NetAmount    float64 `json:"net_amount"`
}

// HSNJobOrderResult is the full API response for HSN job order report.
type HSNJobOrderResult struct {
	Data          []HSNJobOrderRow `json:"data"`
	Total         int              `json:"total"`
	TotalQuantity float64          `json:"total_quantity"`
	TotalAmount   float64          `json:"total_amount"`
}

// Filter holds query parameters common to all HSN reports.
type Filter struct {
	HSNCode  string // required
	FromDate string // YYYY-MM-DD (optional)
	ToDate   string // YYYY-MM-DD (optional)
}
