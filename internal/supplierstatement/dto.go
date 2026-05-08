package supplierstatement

// SupplierInfo holds the supplier's profile shown at the top of the statement.
type SupplierInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Address      string `json:"address"`
	GSTNumber    string `json:"gst_number"`
	SupplierCode string `json:"supplier_code"`
}

// TransactionType classifies each row in the statement ledger.
const (
	TxnInvoice = "INVOICE" // purchase invoice — we owe supplier
	TxnPayment = "PAYMENT" // payment made to supplier
	TxnReturn  = "RETURN"  // purchase return — supplier owes us
)

// StatementLine is one chronological ledger entry.
type StatementLine struct {
	Date           string  `json:"date"`            // DD/MM/YYYY
	Type           string  `json:"type"`            // INVOICE | PAYMENT | RETURN
	RefNumber      string  `json:"ref_number"`      // invoice#, payment ref, return#
	Description    string  `json:"description"`     // human-readable summary
	Debit          float64 `json:"debit"`           // amount added to balance (invoice)
	Credit         float64 `json:"credit"`          // amount deducted from balance (payment/return)
	RunningBalance float64 `json:"running_balance"` // cumulative balance after this line
	// Extra context per type
	InvoiceID     string `json:"invoice_id,omitempty"`
	PaymentID     string `json:"payment_id,omitempty"`
	ReturnID      string `json:"return_id,omitempty"`
	PaymentMethod string `json:"payment_method,omitempty"` // CASH, UPI, etc.
	GRNNumber     string `json:"grn_number,omitempty"`
	Location      string `json:"location,omitempty"` // warehouse/branch name
}

// Summary holds aggregated totals for the statement period.
type Summary struct {
	TotalInvoiced  float64 `json:"total_invoiced"`  // sum of all invoice debits
	TotalPaid      float64 `json:"total_paid"`      // sum of all payment credits
	TotalReturned  float64 `json:"total_returned"`  // sum of all return credits
	OpeningBalance float64 `json:"opening_balance"` // balance before date_from
	ClosingBalance float64 `json:"closing_balance"` // opening + invoiced - paid - returned
	OverdueAmount  float64 `json:"overdue_amount"`  // sum of unpaid invoices past due date (PENDING status)
	InvoiceCount   int     `json:"invoice_count"`
	PaymentCount   int     `json:"payment_count"`
	ReturnCount    int     `json:"return_count"`
}

// StatementResponse is the full API response.
type StatementResponse struct {
	Supplier     SupplierInfo    `json:"supplier"`
	DateFrom     string          `json:"date_from"` // DD/MM/YYYY
	DateTo       string          `json:"date_to"`   // DD/MM/YYYY
	Summary      Summary         `json:"summary"`
	Transactions []StatementLine `json:"transactions"` // chronological order
}
