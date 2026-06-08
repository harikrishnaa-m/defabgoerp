package salesreport

// SalesReportRow is one row in the report — one invoice (or credit note).
// Payment amounts are broken out into separate nullable columns.
// Credit notes (exchanges) appear as negative rows (channel = "CREDIT_NOTE").
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
	Page          int
	Limit         int
}
