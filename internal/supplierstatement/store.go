package supplierstatement

import (
	"database/sql"
	"fmt"
	"sort"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get builds the full supplier statement for a given supplier and date range.
func (s *Store) Get(supplierID, dateFrom, dateTo string) (*StatementResponse, error) {
	// ── 1. Supplier info ──────────────────────────────────────────────────
	var sup SupplierInfo
	err := s.db.QueryRow(`
		SELECT
			id::text,
			COALESCE(name, ''),
			COALESCE(phone, ''),
			COALESCE(email, ''),
			COALESCE(address, ''),
			COALESCE(gst_number, ''),
			COALESCE(supplier_code, '')
		FROM suppliers
		WHERE id = $1
	`, supplierID).Scan(
		&sup.ID, &sup.Name, &sup.Phone, &sup.Email,
		&sup.Address, &sup.GSTNumber, &sup.SupplierCode,
	)
	if err != nil {
		return nil, fmt.Errorf("supplier not found")
	}

	// ── 2. Opening balance (all debits - all credits BEFORE dateFrom) ──────
	var openingInvoiced, openingPaid, openingReturned float64
	s.db.QueryRow(`
		SELECT COALESCE(SUM(net_amount), 0)
		FROM purchase_invoices
		WHERE supplier_id = $1 AND invoice_date < $2::date
	`, supplierID, dateFrom).Scan(&openingInvoiced)

	s.db.QueryRow(`
		SELECT COALESCE(SUM(sp.amount), 0)
		FROM supplier_payments sp
		JOIN purchase_invoices pi ON pi.id = sp.purchase_invoice_id
		WHERE pi.supplier_id = $1 AND sp.paid_at < $2::date
	`, supplierID, dateFrom).Scan(&openingPaid)

	s.db.QueryRow(`
		SELECT COALESCE(SUM(net_amount), 0)
		FROM purchase_returns
		WHERE supplier_id = $1 AND pr_date < $2::date
	`, supplierID, dateFrom).Scan(&openingReturned)

	openingBalance := openingInvoiced - openingPaid - openingReturned

	// ── 3. Invoices in period ─────────────────────────────────────────────
	invRows, err := s.db.Query(`
		SELECT
			pi.id::text,
			TO_CHAR(pi.invoice_date, 'DD/MM/YYYY'),
			COALESCE(pi.invoice_number, ''),
			COALESCE(pi.net_amount, 0),
			COALESCE(pi.notes, ''),
			COALESCE(w.name, ''),
			COALESCE(gr.grn_number, '')
		FROM purchase_invoices pi
		LEFT JOIN warehouses w    ON w.id  = pi.warehouse_id
		LEFT JOIN goods_receipts gr ON gr.purchase_order_id = pi.purchase_order_id
		WHERE pi.supplier_id = $1
		  AND pi.invoice_date >= $2::date
		  AND pi.invoice_date <= $3::date
		ORDER BY pi.invoice_date, pi.created_at
	`, supplierID, dateFrom, dateTo)
	if err != nil {
		return nil, fmt.Errorf("query invoices: %w", err)
	}
	defer invRows.Close()

	type rawLine struct {
		line    StatementLine
		sortKey string // date string for stable sort
	}

	var lines []rawLine
	var totalInvoiced float64
	invoiceCount := 0

	for invRows.Next() {
		var l StatementLine
		var notes string
		l.Type = TxnInvoice
		if err := invRows.Scan(
			&l.InvoiceID, &l.Date, &l.RefNumber,
			&l.Debit, &notes, &l.Location, &l.GRNNumber,
		); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		if notes != "" {
			l.Description = "Purchase Invoice — " + notes
		} else {
			l.Description = "Purchase Invoice"
		}
		totalInvoiced += l.Debit
		invoiceCount++
		lines = append(lines, rawLine{line: l, sortKey: l.Date + l.RefNumber})
	}

	// ── 4. Payments in period ─────────────────────────────────────────────
	payRows, err := s.db.Query(`
		SELECT
			sp.id::text,
			TO_CHAR(sp.paid_at, 'DD/MM/YYYY'),
			COALESCE(sp.reference, ''),
			sp.amount,
			sp.payment_method,
			pi.invoice_number
		FROM supplier_payments sp
		JOIN purchase_invoices pi ON pi.id = sp.purchase_invoice_id
		WHERE pi.supplier_id = $1
		  AND sp.paid_at::date >= $2::date
		  AND sp.paid_at::date <= $3::date
		ORDER BY sp.paid_at, sp.created_at
	`, supplierID, dateFrom, dateTo)
	if err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
	}
	defer payRows.Close()

	var totalPaid float64
	paymentCount := 0

	for payRows.Next() {
		var l StatementLine
		var invoiceNum string
		l.Type = TxnPayment
		if err := payRows.Scan(
			&l.PaymentID, &l.Date, &l.RefNumber,
			&l.Credit, &l.PaymentMethod, &invoiceNum,
		); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		l.Description = "Payment — " + l.PaymentMethod
		if invoiceNum != "" {
			l.Description += " (Bill# " + invoiceNum + ")"
		}
		totalPaid += l.Credit
		paymentCount++
		lines = append(lines, rawLine{line: l, sortKey: l.Date + l.PaymentID})
	}

	// ── 5. Returns in period ──────────────────────────────────────────────
	retRows, err := s.db.Query(`
		SELECT
			pr.id::text,
			TO_CHAR(pr.pr_date, 'DD/MM/YYYY'),
			pr.pr_number,
			COALESCE(pr.net_amount, 0),
			COALESCE(pr.reason, ''),
			COALESCE(pi.invoice_number, '')
		FROM purchase_returns pr
		LEFT JOIN purchase_invoices pi ON pi.id = pr.purchase_invoice_id
		WHERE pr.supplier_id = $1
		  AND pr.pr_date >= $2::date
		  AND pr.pr_date <= $3::date
		ORDER BY pr.pr_date, pr.created_at
	`, supplierID, dateFrom, dateTo)
	if err != nil {
		return nil, fmt.Errorf("query returns: %w", err)
	}
	defer retRows.Close()

	var totalReturned float64
	returnCount := 0

	for retRows.Next() {
		var l StatementLine
		var reason, invoiceNum string
		l.Type = TxnReturn
		if err := retRows.Scan(
			&l.ReturnID, &l.Date, &l.RefNumber,
			&l.Credit, &reason, &invoiceNum,
		); err != nil {
			return nil, fmt.Errorf("scan return: %w", err)
		}
		l.Description = "Purchase Return"
		if invoiceNum != "" {
			l.Description += " (Bill# " + invoiceNum + ")"
		}
		if reason != "" {
			l.Description += " — " + reason
		}
		totalReturned += l.Credit
		returnCount++
		lines = append(lines, rawLine{line: l, sortKey: l.Date + l.ReturnID})
	}

	// ── 6. Overdue amount (unpaid/partial invoices regardless of period) ──
	var overdueAmount float64
	s.db.QueryRow(`
		SELECT COALESCE(SUM(net_amount - paid_amount), 0)
		FROM purchase_invoices
		WHERE supplier_id = $1
		  AND status IN ('PENDING', 'PARTIAL')
		  AND invoice_date <= NOW()
	`, supplierID).Scan(&overdueAmount)
	if overdueAmount < 0 {
		overdueAmount = 0
	}

	// ── 7. Sort all lines chronologically ────────────────────────────────
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].sortKey < lines[j].sortKey
	})

	// ── 8. Compute running balance ────────────────────────────────────────
	running := openingBalance
	result := make([]StatementLine, len(lines))
	for i, rl := range lines {
		l := rl.line
		running += l.Debit
		running -= l.Credit
		l.RunningBalance = running
		result[i] = l
	}

	closingBalance := openingBalance + totalInvoiced - totalPaid - totalReturned

	return &StatementResponse{
		Supplier: sup,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Summary: Summary{
			TotalInvoiced:  totalInvoiced,
			TotalPaid:      totalPaid,
			TotalReturned:  totalReturned,
			OpeningBalance: openingBalance,
			ClosingBalance: closingBalance,
			OverdueAmount:  overdueAmount,
			InvoiceCount:   invoiceCount,
			PaymentCount:   paymentCount,
			ReturnCount:    returnCount,
		},
		Transactions: result,
	}, nil
}
