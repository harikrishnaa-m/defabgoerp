package salesreport

// SalesReportRow is one row in the report — one invoice (or credit note).
// Payment amounts are broken out into separate nullable columns.
// Credit notes (exchanges) appear as negative rows (channel = "CREDIT_NOTE").
// VariantName and Quantity are populated only when variant_code filter is active.
type SalesReportRow struct {
	ID              string   `json:"id"`
	InvoiceNumber   string   `json:"invoice_number"`
	Date            string   `json:"date"`
	CustomerName    string   `json:"customer_name"`
	Channel         string   `json:"channel"`
	NetAmount       float64  `json:"net_amount"`
	GSTAmount       float64  `json:"gst_amount"`
	CGSTAmount      float64  `json:"cgst_amount"`
	SGSTAmount      float64  `json:"sgst_amount"`
	Cash            *float64 `json:"cash"`
	Card            *float64 `json:"card"`
	UPI             *float64 `json:"upi"`
	BankTransfer    *float64 `json:"bank_transfer"`
	ExchangeCredit  *float64 `json:"exchange_credit"`
	Location        string   `json:"location"`
	SalespersonName string   `json:"salesperson_name"`
	CreatedByName   string   `json:"created_by_name"`
	VariantName     string   `json:"variant_name,omitempty"`   // populated when variant_code filter is active
	Quantity        float64  `json:"quantity,omitempty"`       // populated when variant_code filter is active
	SupplierNames   *string  `json:"supplier_names,omitempty"` // null when no filter; "" when filter active but no supplier found
}

// ReportTotals holds column-level sums for the entire filtered result set.
type ReportTotals struct {
	NetAmount      float64  `json:"net_amount"`
	GSTAmount      float64  `json:"gst_amount"`
	CGSTAmount     float64  `json:"cgst_amount"`
	SGSTAmount     float64  `json:"sgst_amount"`
	Cash           *float64 `json:"cash"`
	Card           *float64 `json:"card"`
	UPI            *float64 `json:"upi"`
	BankTransfer   *float64 `json:"bank_transfer"`
	ExchangeCredit *float64 `json:"exchange_credit"`
}

// ReportResult is the full API response.
type ReportResult struct {
	Data           []SalesReportRow `json:"data"`
	Total          int              `json:"total"`
	Page           int              `json:"page"`
	Limit          int              `json:"limit"`
	TotalPages     int              `json:"total_pages"`
	TotalNetAmount float64          `json:"total_net_amount"` // kept for backwards compat
	Totals         ReportTotals     `json:"totals"`
}

// Filter holds all query parameters for the report.
type Filter struct {
	BranchID      string
	FromDate      string // YYYY-MM-DD
	ToDate        string // YYYY-MM-DD
	SalespersonID string
	CreatedByID   string
	PaymentType   string // CASH, UPI, CARD, BANK_TRANSFER
	Channel       string // STORE, ONLINE, EXCHANGE
	VariantCode   string // filter by variant code (supports multiple variants with same code)
	Page          int
	Limit         int
}

// DetailedReportRow is one item line in the detailed sales report.
// Invoice-level fields repeat for every item on that invoice.
type DetailedReportRow struct {
	InvoiceNumber  string   `json:"invoice_number"`
	Date           string   `json:"date"`
	CustomerName   string   `json:"customer_name"`
	Branch         string   `json:"branch"`
	Salesperson    string   `json:"salesperson"`
	Status         string   `json:"status"`
	PaymentMethods string   `json:"payment_methods"`
	PaymentAmounts string   `json:"payment_amounts"`
	PaymentRefs    string   `json:"payment_references"`
	Cash           *float64 `json:"cash"`
	DebitCard      *float64 `json:"debit_card"`
	CreditCard     *float64 `json:"credit_card"`
	UPI            *float64 `json:"upi"`
	BankTransfer   *float64 `json:"bank_transfer"`
	ExchangeCredit *float64 `json:"exchange_credit"`
	SubAmount      float64  `json:"sub_amount"`
	DiscountAmount float64  `json:"discount_amount"`
	BillDiscount   float64  `json:"bill_discount"`
	CGST           float64  `json:"cgst"`
	SGST           float64  `json:"sgst"`
	TotalGST       float64  `json:"total_gst"`
	RoundOff       float64  `json:"round_off"`
	NetAmount      float64  `json:"net_amount"`
	VariantCode    string   `json:"variant_code"`
	ItemName       string   `json:"item_name"`
	SKU            string   `json:"sku"`
	HSNCode        string   `json:"hsn_code"`
	Quantity       float64  `json:"quantity"`
	UnitPrice      float64  `json:"unit_price"`
	ItemDiscount   float64  `json:"item_discount"`
	ItemGSTPercent float64  `json:"item_gst_percent"`
	ItemCGST       float64  `json:"item_cgst"`
	ItemSGST       float64  `json:"item_sgst"`
	ItemTotalGST   float64  `json:"item_total_gst"`
	ItemTotal      float64  `json:"item_total"`
}

// DetailedTotals holds column-level sums for the entire filtered detailed result set.
// Invoice-level fields (net_amount, gst, discount, payment totals) are summed once per unique invoice.
// Item-level fields (quantity, item_total, item_gst) are summed across all line items.
type DetailedTotals struct {
	NetAmount      float64  `json:"net_amount"`
	DiscountAmount float64  `json:"discount_amount"`
	BillDiscount   float64  `json:"bill_discount"`
	CGST           float64  `json:"cgst"`
	SGST           float64  `json:"sgst"`
	TotalGST       float64  `json:"total_gst"`
	RoundOff       float64  `json:"round_off"`
	Cash           *float64 `json:"cash"`
	DebitCard      *float64 `json:"debit_card"`
	CreditCard     *float64 `json:"credit_card"`
	UPI            *float64 `json:"upi"`
	BankTransfer   *float64 `json:"bank_transfer"`
	ExchangeCredit *float64 `json:"exchange_credit"`
	Quantity       float64  `json:"quantity"`
	ItemTotal      float64  `json:"item_total"`
	ItemTotalGST   float64  `json:"item_total_gst"`
}

// DetailedReportResult is the full API response for the detailed report.
type DetailedReportResult struct {
	Data       []DetailedReportRow `json:"data"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	Limit      int                 `json:"limit"`
	TotalPages int                 `json:"total_pages"`
	Totals     DetailedTotals      `json:"totals"`
}
