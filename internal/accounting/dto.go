package accounting

// ── Ledger Account ──

type CreateLedgerAccountInput struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	AccountGroupID string `json:"account_group_id"`
	Nature         string `json:"nature"` // DEBIT or CREDIT
	Description    string `json:"description"`
}

// ── Financial Year ──

type CreateFinancialYearInput struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`
}

// ── Manual Voucher (Journal) ──

type CreateVoucherInput struct {
	VoucherType string             `json:"voucher_type"` // JOURNAL, CONTRA
	VoucherDate string             `json:"voucher_date"` // YYYY-MM-DD
	Narration   string             `json:"narration"`
	BranchID    string             `json:"branch_id"`
	Lines       []VoucherLineInput `json:"lines"`
}

type VoucherLineInput struct {
	LedgerAccountID string  `json:"ledger_account_id"`
	Debit           float64 `json:"debit"`
	Credit          float64 `json:"credit"`
	Narration       string  `json:"narration"`
}

// ── Account Group ──

type CreateAccountGroupInput struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id"`
	Nature   string  `json:"nature"` // ASSET, LIABILITY, INCOME, EXPENSE, EQUITY
}
