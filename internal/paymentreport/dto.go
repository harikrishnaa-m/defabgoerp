package paymentreport

// PaymentReportRow is one row in the report — one payment transaction.
type PaymentReportRow struct {
	ID            string  `json:"id"`
	TxnType       string  `json:"txn_type"`       // CASH, UPI, CARD, DEBIT_CARD, CREDIT_CARD, BANK_TRANSFER
	TransactionNo string  `json:"transaction_no"` // reference field
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"` // DD/MM/YYYY
	InvoiceNumber string  `json:"invoice_number"`
	CustomerName  string  `json:"customer_name"`
	Location      string  `json:"location"`
}

// ReportResult is the full API response.
type ReportResult struct {
	Data        []PaymentReportRow `json:"data"`
	Total       int                `json:"total"`
	Page        int                `json:"page"`
	Limit       int                `json:"limit"`
	TotalPages  int                `json:"total_pages"`
	TotalAmount float64            `json:"total_amount"`
}

// Filter holds all query parameters.
type Filter struct {
	BranchID      string
	FromDate      string // YYYY-MM-DD
	ToDate        string // YYYY-MM-DD
	PaymentMethod string // CASH | UPI | CARD | DEBIT_CARD | CREDIT_CARD | BANK_TRANSFER  (empty = all)
	Page          int
	Limit         int
}
