package salesreport

// SalesReportRow is one row in the report — one payment transaction.
type SalesReportRow struct {
	ID                    string  `json:"id"`
	InvoiceNumber         string  `json:"invoice_number"`
	Date                  string  `json:"date"`
	CustomerName          string  `json:"customer_name"`
	NetAmount             float64 `json:"net_amount"`
	PaymentMethod         string  `json:"payment_method"`
	Location              string  `json:"location"`
	SalespersonName       string  `json:"salesperson_name"`
	CreatedByName         string  `json:"created_by_name"`
	Channel               string  `json:"channel"`
	IsReturned            bool    `json:"is_returned"`
	ExchangeInvoiceNumber string  `json:"exchange_invoice_number"`
}

// ReportResult is the full API response.
type ReportResult struct {
	Data           []SalesReportRow `json:"data"`
	Total          int              `json:"total"`
	Page           int              `json:"page"`
	Limit          int              `json:"limit"`
	TotalPages     int              `json:"total_pages"`
	TotalNetAmount float64          `json:"total_net_amount"`
}

// Filter holds all query parameters for the report.
type Filter struct {
	BranchID      string
	FromDate      string // YYYY-MM-DD
	ToDate        string // YYYY-MM-DD
	SalespersonID string
	CreatedByID   string
	PaymentType   string // CASH, UPI, CARD, BANK_TRANSFER
	Channel       string // STORE, ONLINE
	Page          int
	Limit         int
}
