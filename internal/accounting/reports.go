package accounting

import (
	"fmt"
)

// ════════════════════════════════════════════
// Report row types
// ════════════════════════════════════════════

type TrialBalanceRow struct {
	AccountID   string  `json:"account_id"`
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Nature      string  `json:"nature"`
	TotalDebit  float64 `json:"total_debit"`
	TotalCredit float64 `json:"total_credit"`
	Balance     float64 `json:"balance"` // positive = normal side, negative = opposite
}

type LedgerEntry struct {
	VoucherID     string  `json:"voucher_id"`
	VoucherNumber string  `json:"voucher_number"`
	VoucherType   string  `json:"voucher_type"`
	VoucherDate   string  `json:"voucher_date"`
	Narration     string  `json:"narration"`
	Debit         float64 `json:"debit"`
	Credit        float64 `json:"credit"`
	RunningBal    float64 `json:"running_balance"`
}

type ProfitLossSection struct {
	AccountID   string  `json:"account_id"`
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Amount      float64 `json:"amount"`
}

type BalanceSheetRow struct {
	AccountID   string  `json:"account_id"`
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	GroupName   string  `json:"group_name"`
	Nature      string  `json:"nature"`
	Balance     float64 `json:"balance"`
}

type DayBookEntry struct {
	VoucherID     string  `json:"voucher_id"`
	VoucherNumber string  `json:"voucher_number"`
	VoucherType   string  `json:"voucher_type"`
	Narration     string  `json:"narration"`
	TotalDebit    float64 `json:"total_debit"`
	TotalCredit   float64 `json:"total_credit"`
	IsCancelled   bool    `json:"is_cancelled"`
}

// ════════════════════════════════════════════
// Trial Balance
// ════════════════════════════════════════════

func (s *Store) TrialBalance(asOf string) ([]TrialBalanceRow, error) {
	query := `
		SELECT la.id, la.code, la.name, la.nature,
		       COALESCE(SUM(vl.debit), 0) AS total_debit,
		       COALESCE(SUM(vl.credit), 0) AS total_credit
		FROM ledger_accounts la
		LEFT JOIN voucher_lines vl ON vl.ledger_account_id = la.id
		LEFT JOIN vouchers v ON v.id = vl.voucher_id
		    AND v.is_cancelled = FALSE
	`
	args := []interface{}{}
	if asOf != "" {
		query += " AND v.voucher_date <= $1"
		args = append(args, asOf)
	}
	query += `
		GROUP BY la.id, la.code, la.name, la.nature
		HAVING COALESCE(SUM(vl.debit), 0) != 0
		    OR COALESCE(SUM(vl.credit), 0) != 0
		ORDER BY la.code
	`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TrialBalanceRow
	for rows.Next() {
		var r TrialBalanceRow
		if err := rows.Scan(&r.AccountID, &r.AccountCode, &r.AccountName, &r.Nature,
			&r.TotalDebit, &r.TotalCredit); err != nil {
			return nil, err
		}
		r.Balance = r.TotalDebit - r.TotalCredit
		result = append(result, r)
	}
	return result, nil
}

// ════════════════════════════════════════════
// Ledger (single account transactions)
// ════════════════════════════════════════════

func (s *Store) Ledger(accountID, from, to string) ([]LedgerEntry, error) {
	where := "WHERE vl.ledger_account_id = $1 AND v.is_cancelled = FALSE"
	args := []interface{}{accountID}
	idx := 2

	if from != "" {
		where += fmt.Sprintf(" AND v.voucher_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		where += fmt.Sprintf(" AND v.voucher_date <= $%d", idx)
		args = append(args, to)
		idx++
	}

	query := fmt.Sprintf(`
		SELECT v.id, v.voucher_number, v.voucher_type, v.voucher_date::text,
		       COALESCE(v.narration,''), vl.debit, vl.credit
		FROM voucher_lines vl
		JOIN vouchers v ON v.id = vl.voucher_id
		%s
		ORDER BY v.voucher_date, v.created_at
	`, where)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LedgerEntry
	var running float64
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.VoucherID, &e.VoucherNumber, &e.VoucherType,
			&e.VoucherDate, &e.Narration, &e.Debit, &e.Credit); err != nil {
			return nil, err
		}
		running += e.Debit - e.Credit
		e.RunningBal = running
		entries = append(entries, e)
	}
	return entries, nil
}

// ════════════════════════════════════════════
// Profit & Loss
// ════════════════════════════════════════════

func (s *Store) ProfitAndLoss(from, to string) (map[string]interface{}, error) {
	joinCond := "AND v.is_cancelled = FALSE"
	args := []interface{}{}
	idx := 1

	if from != "" {
		joinCond += fmt.Sprintf(" AND v.voucher_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		joinCond += fmt.Sprintf(" AND v.voucher_date <= $%d", idx)
		args = append(args, to)
		idx++
	}

	query := fmt.Sprintf(`
		SELECT la.id, la.code, la.name, ag.nature,
		       COALESCE(SUM(vl.debit), 0), COALESCE(SUM(vl.credit), 0)
		FROM ledger_accounts la
		JOIN account_groups ag ON ag.id = la.account_group_id
		LEFT JOIN voucher_lines vl ON vl.ledger_account_id = la.id
		LEFT JOIN vouchers v ON v.id = vl.voucher_id %s
		WHERE ag.nature IN ('INCOME', 'EXPENSE')
		GROUP BY la.id, la.code, la.name, ag.nature
		HAVING COALESCE(SUM(vl.debit), 0) != 0 OR COALESCE(SUM(vl.credit), 0) != 0
		ORDER BY ag.nature, la.code
	`, joinCond)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incomeItems []ProfitLossSection
	var expenseItems []ProfitLossSection
	var totalIncome, totalExpense float64

	for rows.Next() {
		var id, code, name, nature string
		var debit, credit float64
		if err := rows.Scan(&id, &code, &name, &nature, &debit, &credit); err != nil {
			return nil, err
		}

		if nature == "INCOME" {
			amount := credit - debit
			incomeItems = append(incomeItems, ProfitLossSection{
				AccountID: id, AccountCode: code, AccountName: name, Amount: amount,
			})
			totalIncome += amount
		} else {
			amount := debit - credit
			expenseItems = append(expenseItems, ProfitLossSection{
				AccountID: id, AccountCode: code, AccountName: name, Amount: amount,
			})
			totalExpense += amount
		}
	}

	return map[string]interface{}{
		"income":        incomeItems,
		"total_income":  totalIncome,
		"expenses":      expenseItems,
		"total_expense": totalExpense,
		"net_profit":    totalIncome - totalExpense,
	}, nil
}

// ════════════════════════════════════════════
// Balance Sheet
// ════════════════════════════════════════════

func (s *Store) BalanceSheet(asOf string) (map[string]interface{}, error) {
	query := `
		SELECT la.id, la.code, la.name, ag.name, ag.nature,
		       COALESCE(SUM(vl.debit), 0), COALESCE(SUM(vl.credit), 0)
		FROM ledger_accounts la
		JOIN account_groups ag ON ag.id = la.account_group_id
		LEFT JOIN voucher_lines vl ON vl.ledger_account_id = la.id
		LEFT JOIN vouchers v ON v.id = vl.voucher_id
		    AND v.is_cancelled = FALSE
	`
	args := []interface{}{}
	if asOf != "" {
		query += " AND v.voucher_date <= $1"
		args = append(args, asOf)
	}
	query += `
		WHERE ag.nature IN ('ASSET', 'LIABILITY', 'EQUITY')
		GROUP BY la.id, la.code, la.name, ag.name, ag.nature
		HAVING COALESCE(SUM(vl.debit), 0) != 0 OR COALESCE(SUM(vl.credit), 0) != 0
		ORDER BY ag.nature, la.code
	`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets, liabilities, equity []BalanceSheetRow
	var totalAssets, totalLiabilities, totalEquity float64

	for rows.Next() {
		var r BalanceSheetRow
		var debit, credit float64
		if err := rows.Scan(&r.AccountID, &r.AccountCode, &r.AccountName, &r.GroupName, &r.Nature,
			&debit, &credit); err != nil {
			return nil, err
		}

		switch r.Nature {
		case "ASSET":
			r.Balance = debit - credit
			assets = append(assets, r)
			totalAssets += r.Balance
		case "LIABILITY":
			r.Balance = credit - debit
			liabilities = append(liabilities, r)
			totalLiabilities += r.Balance
		case "EQUITY":
			r.Balance = credit - debit
			equity = append(equity, r)
			totalEquity += r.Balance
		}
	}

	return map[string]interface{}{
		"assets":            assets,
		"total_assets":      totalAssets,
		"liabilities":       liabilities,
		"total_liabilities": totalLiabilities,
		"equity":            equity,
		"total_equity":      totalEquity,
	}, nil
}

// ════════════════════════════════════════════
// Day Book
// ════════════════════════════════════════════

func (s *Store) DayBook(date string) ([]DayBookEntry, error) {
	rows, err := s.db.Query(`
		SELECT v.id, v.voucher_number, v.voucher_type, COALESCE(v.narration,''),
		       COALESCE(SUM(vl.debit),0), COALESCE(SUM(vl.credit),0), v.is_cancelled
		FROM vouchers v
		LEFT JOIN voucher_lines vl ON vl.voucher_id = v.id
		WHERE v.voucher_date = $1
		GROUP BY v.id
		ORDER BY v.created_at
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DayBookEntry
	for rows.Next() {
		var e DayBookEntry
		if err := rows.Scan(&e.VoucherID, &e.VoucherNumber, &e.VoucherType,
			&e.Narration, &e.TotalDebit, &e.TotalCredit, &e.IsCancelled); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
