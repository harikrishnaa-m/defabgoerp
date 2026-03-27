package accounting

import "time"

// Well-known ledger account IDs (match migration seed data)
const (
	LedgerCash             = "b0000000-0000-0000-0000-000000000001"
	LedgerBank             = "b0000000-0000-0000-0000-000000000002"
	LedgerAccountsReceiv   = "b0000000-0000-0000-0000-000000000003"
	LedgerInventory        = "b0000000-0000-0000-0000-000000000004"
	LedgerUPI              = "b0000000-0000-0000-0000-000000000005"
	LedgerCard             = "b0000000-0000-0000-0000-000000000006"
	LedgerAccountsPayable  = "b0000000-0000-0000-0000-000000000010"
	LedgerGSTPayable       = "b0000000-0000-0000-0000-000000000011"
	LedgerGSTReceivable    = "b0000000-0000-0000-0000-000000000012"
	LedgerSalesRevenue     = "b0000000-0000-0000-0000-000000000020"
	LedgerCOGS             = "b0000000-0000-0000-0000-000000000030"
	LedgerPurchaseExpense  = "b0000000-0000-0000-0000-000000000031"
	LedgerDiscountAllowed  = "b0000000-0000-0000-0000-000000000040"
	LedgerDiscountReceived = "b0000000-0000-0000-0000-000000000041"
)

// Voucher types (Tally vocabulary)
const (
	VoucherTypeSales    = "SALES"
	VoucherTypePurchase = "PURCHASE"
	VoucherTypeReceipt  = "RECEIPT"
	VoucherTypePayment  = "PAYMENT"
	VoucherTypeJournal  = "JOURNAL"
	VoucherTypeContra   = "CONTRA"
)

// Ref types — link vouchers back to source records
const (
	RefSalesInvoice    = "sales_invoice"
	RefSalesPayment    = "sales_payment"
	RefPurchaseInvoice = "purchase_invoice"
	RefSupplierPayment = "supplier_payment"
)

// AccountGroup represents a node in the chart-of-accounts tree.
type AccountGroup struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ParentID   *string   `json:"parent_id"`
	ParentName *string   `json:"parent_name,omitempty"`
	Nature     string    `json:"nature"`
	CreatedAt  time.Time `json:"created_at"`
}

// LedgerAccount is an individual account in the chart of accounts.
type LedgerAccount struct {
	ID             string    `json:"id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	AccountGroupID string    `json:"account_group_id"`
	GroupName      string    `json:"group_name,omitempty"`
	Nature         string    `json:"nature"`
	IsSystem       bool      `json:"is_system"`
	IsActive       bool      `json:"is_active"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}

// FinancialYear represents a fiscal period.
type FinancialYear struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	StartDate string    `json:"start_date"`
	EndDate   string    `json:"end_date"`
	IsActive  bool      `json:"is_active"`
	IsClosed  bool      `json:"is_closed"`
	CreatedAt time.Time `json:"created_at"`
}

// Voucher is the Tally-style journal entry header.
type Voucher struct {
	ID                string        `json:"id"`
	VoucherNumber     string        `json:"voucher_number"`
	VoucherType       string        `json:"voucher_type"`
	VoucherDate       string        `json:"voucher_date"`
	Narration         string        `json:"narration"`
	RefType           string        `json:"ref_type"`
	RefID             string        `json:"ref_id"`
	FinancialYearID   string        `json:"financial_year_id"`
	FinancialYearName string        `json:"financial_year_name,omitempty"`
	BranchID          string        `json:"branch_id"`
	BranchName        string        `json:"branch_name,omitempty"`
	IsCancelled       bool          `json:"is_cancelled"`
	CreatedBy         string        `json:"created_by"`
	CreatedAt         time.Time     `json:"created_at"`
	Lines             []VoucherLine `json:"lines"`
	TotalDebit        float64       `json:"total_debit"`
	TotalCredit       float64       `json:"total_credit"`
}

// VoucherLine is a single debit or credit entry inside a voucher.
type VoucherLine struct {
	ID              string  `json:"id"`
	VoucherID       string  `json:"voucher_id,omitempty"`
	LedgerAccountID string  `json:"ledger_account_id"`
	AccountName     string  `json:"account_name,omitempty"`
	AccountCode     string  `json:"account_code,omitempty"`
	Debit           float64 `json:"debit"`
	Credit          float64 `json:"credit"`
	Narration       string  `json:"narration"`
}

// PaymentLedgerMap maps payment method strings to ledger account IDs.
func PaymentLedgerMap(method string) string {
	switch method {
	case "CASH":
		return LedgerCash
	case "BANK_TRANSFER":
		return LedgerBank
	case "UPI":
		return LedgerUPI
	case "CARD":
		return LedgerCard
	default:
		return LedgerCash
	}
}
