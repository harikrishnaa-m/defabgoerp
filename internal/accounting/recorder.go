package accounting

import (
	"database/sql"
	"fmt"
)

// Recorder reads from existing ERP tables (sales_invoices, purchase_invoices, etc.)
// and writes double-entry vouchers into the accounting tables.
// It never modifies any existing table — only reads from them and writes to vouchers/voucher_lines.
type Recorder struct {
	db    *sql.DB
	store *Store
}

func NewRecorder(db *sql.DB, store *Store) *Recorder {
	return &Recorder{db: db, store: store}
}

// ════════════════════════════════════════════
// Sales Invoice → SALES voucher
// ════════════════════════════════════════════
//
// Double-entry:
//   DR  Accounts Receivable      net_amount
//   CR  Sales Revenue            (sub_amount - discounts)
//   CR  GST Payable              gst_amount
//   DR  Discount Allowed         discounts   (if any)
//
// If payments were recorded at billing time:
//   DR  Cash / Bank / UPI / Card   paid_amount
//   CR  Accounts Receivable        paid_amount

func (r *Recorder) RecordSalesInvoice(salesInvoiceID, userID string) error {
	exists, err := r.store.VoucherExistsForRef(RefSalesInvoice, salesInvoiceID)
	if err != nil {
		return fmt.Errorf("check existing voucher: %w", err)
	}
	if exists {
		return nil // idempotent
	}

	var inv struct {
		ID, InvoiceNumber                       string
		BranchID                                sql.NullString
		SubAmount, DiscountAmount, BillDiscount float64
		GSTAmount, NetAmount, PaidAmount        float64
		InvoiceDate                             string
	}
	err = r.db.QueryRow(`
		SELECT id, invoice_number, branch_id,
		       sub_amount, discount_amount, bill_discount,
		       gst_amount, net_amount, paid_amount,
		       invoice_date::date
		FROM sales_invoices WHERE id = $1
	`, salesInvoiceID).Scan(
		&inv.ID, &inv.InvoiceNumber, &inv.BranchID,
		&inv.SubAmount, &inv.DiscountAmount, &inv.BillDiscount,
		&inv.GSTAmount, &inv.NetAmount, &inv.PaidAmount, &inv.InvoiceDate,
	)
	if err != nil {
		return fmt.Errorf("read sales invoice: %w", err)
	}

	totalDiscount := inv.DiscountAmount + inv.BillDiscount

	lines := []VoucherLine{
		{LedgerAccountID: LedgerAccountsReceiv, Debit: inv.NetAmount, Narration: "Customer receivable"},
		{LedgerAccountID: LedgerSalesRevenue, Credit: inv.SubAmount, Narration: "Sales revenue"},
	}
	if inv.GSTAmount > 0 {
		lines = append(lines, VoucherLine{
			LedgerAccountID: LedgerGSTPayable, Credit: inv.GSTAmount, Narration: "Output GST",
		})
	}
	if totalDiscount > 0 {
		lines = append(lines, VoucherLine{
			LedgerAccountID: LedgerDiscountAllowed, Debit: totalDiscount, Narration: "Discount allowed",
		})
	}

	// Record inline payments (settle receivable immediately)
	if inv.PaidAmount > 0 {
		rows, err := r.db.Query(`
			SELECT payment_method, SUM(amount) FROM sales_payments
			WHERE sales_invoice_id = $1 GROUP BY payment_method
		`, salesInvoiceID)
		if err == nil {
			defer rows.Close()
			var totalSettled float64
			for rows.Next() {
				var method string
				var amt float64
				if err := rows.Scan(&method, &amt); err != nil {
					continue
				}
				ledgerID := PaymentLedgerMap(method)
				lines = append(lines, VoucherLine{
					LedgerAccountID: ledgerID, Debit: amt,
					Narration: method + " payment received",
				})
				totalSettled += amt
			}
			if totalSettled > 0 {
				lines = append(lines, VoucherLine{
					LedgerAccountID: LedgerAccountsReceiv, Credit: totalSettled,
					Narration: "Receivable settled by payment",
				})
			}
		}
	}

	branchID := ""
	if inv.BranchID.Valid {
		branchID = inv.BranchID.String
	}

	return r.store.CreateVoucher(Voucher{
		VoucherType: VoucherTypeSales,
		VoucherDate: inv.InvoiceDate,
		Narration:   "Sales Invoice " + inv.InvoiceNumber,
		RefType:     RefSalesInvoice,
		RefID:       salesInvoiceID,
		BranchID:    branchID,
		CreatedBy:   userID,
		Lines:       lines,
	})
}

// ════════════════════════════════════════════
// Purchase Invoice → PURCHASE voucher
// ════════════════════════════════════════════
//
//   DR  Purchase Expense          sub_amount
//   DR  GST Receivable            gst_amount
//   CR  Accounts Payable          net_amount
//   CR  Discount Received         discount    (if any)

func (r *Recorder) RecordPurchaseInvoice(purchaseInvoiceID, userID string) error {
	exists, err := r.store.VoucherExistsForRef(RefPurchaseInvoice, purchaseInvoiceID)
	if err != nil {
		return fmt.Errorf("check existing voucher: %w", err)
	}
	if exists {
		return nil
	}

	var inv struct {
		ID, InvoiceNumber                string
		SubAmount, DiscountAmount        float64
		GSTAmount, NetAmount, PaidAmount float64
		InvoiceDate                      string
		BranchID                         sql.NullString
	}
	err = r.db.QueryRow(`
		SELECT pi.id, pi.invoice_number,
		       pi.sub_amount, pi.discount_amount,
		       pi.gst_amount, pi.net_amount, pi.paid_amount,
		       pi.invoice_date::date,
		       w.branch_id
		FROM purchase_invoices pi
		LEFT JOIN warehouses w ON w.id = pi.warehouse_id
		WHERE pi.id = $1
	`, purchaseInvoiceID).Scan(
		&inv.ID, &inv.InvoiceNumber,
		&inv.SubAmount, &inv.DiscountAmount,
		&inv.GSTAmount, &inv.NetAmount, &inv.PaidAmount, &inv.InvoiceDate,
		&inv.BranchID,
	)
	if err != nil {
		return fmt.Errorf("read purchase invoice: %w", err)
	}

	lines := []VoucherLine{
		{LedgerAccountID: LedgerPurchaseExpense, Debit: inv.SubAmount, Narration: "Purchase cost"},
		{LedgerAccountID: LedgerAccountsPayable, Credit: inv.NetAmount, Narration: "Supplier payable"},
	}
	if inv.GSTAmount > 0 {
		lines = append(lines, VoucherLine{
			LedgerAccountID: LedgerGSTReceivable, Debit: inv.GSTAmount, Narration: "Input GST credit",
		})
	}
	if inv.DiscountAmount > 0 {
		lines = append(lines, VoucherLine{
			LedgerAccountID: LedgerDiscountReceived, Credit: inv.DiscountAmount, Narration: "Discount from supplier",
		})
	}

	// Record inline payment at invoice creation
	if inv.PaidAmount > 0 {
		rows, err := r.db.Query(`
			SELECT payment_method, SUM(amount) FROM supplier_payments
			WHERE purchase_invoice_id = $1 GROUP BY payment_method
		`, purchaseInvoiceID)
		if err == nil {
			defer rows.Close()
			var totalSettled float64
			for rows.Next() {
				var method string
				var amt float64
				if err := rows.Scan(&method, &amt); err != nil {
					continue
				}
				ledgerID := PaymentLedgerMap(method)
				lines = append(lines, VoucherLine{
					LedgerAccountID: LedgerAccountsPayable, Debit: amt,
					Narration: "Payable settled",
				})
				lines = append(lines, VoucherLine{
					LedgerAccountID: ledgerID, Credit: amt,
					Narration: method + " payment to supplier",
				})
				totalSettled += amt
			}
		}
	}

	branchID := ""
	if inv.BranchID.Valid {
		branchID = inv.BranchID.String
	}

	return r.store.CreateVoucher(Voucher{
		VoucherType: VoucherTypePurchase,
		VoucherDate: inv.InvoiceDate,
		Narration:   "Purchase Invoice " + inv.InvoiceNumber,
		RefType:     RefPurchaseInvoice,
		RefID:       purchaseInvoiceID,
		BranchID:    branchID,
		CreatedBy:   userID,
		Lines:       lines,
	})
}

// ════════════════════════════════════════════
// Sales Payment → RECEIPT voucher
// ════════════════════════════════════════════
//
//   DR  Cash / Bank / UPI / Card   amount
//   CR  Accounts Receivable        amount

func (r *Recorder) RecordSalesPayment(salesPaymentID, userID string) error {
	exists, err := r.store.VoucherExistsForRef(RefSalesPayment, salesPaymentID)
	if err != nil {
		return fmt.Errorf("check existing voucher: %w", err)
	}
	if exists {
		return nil
	}

	var sp struct {
		ID, InvoiceID, Method string
		Amount                float64
		PaidAt                string
	}
	err = r.db.QueryRow(`
		SELECT id, sales_invoice_id, payment_method, amount, paid_at::date
		FROM sales_payments WHERE id = $1
	`, salesPaymentID).Scan(&sp.ID, &sp.InvoiceID, &sp.Method, &sp.Amount, &sp.PaidAt)
	if err != nil {
		return fmt.Errorf("read sales payment: %w", err)
	}

	var invoiceNumber string
	r.db.QueryRow(`SELECT invoice_number FROM sales_invoices WHERE id = $1`, sp.InvoiceID).Scan(&invoiceNumber)

	ledgerID := PaymentLedgerMap(sp.Method)
	lines := []VoucherLine{
		{LedgerAccountID: ledgerID, Debit: sp.Amount, Narration: sp.Method + " received"},
		{LedgerAccountID: LedgerAccountsReceiv, Credit: sp.Amount, Narration: "Receivable settled"},
	}

	return r.store.CreateVoucher(Voucher{
		VoucherType: VoucherTypeReceipt,
		VoucherDate: sp.PaidAt,
		Narration:   "Payment received for " + invoiceNumber,
		RefType:     RefSalesPayment,
		RefID:       salesPaymentID,
		CreatedBy:   userID,
		Lines:       lines,
	})
}

// ════════════════════════════════════════════
// Supplier Payment → PAYMENT voucher
// ════════════════════════════════════════════
//
//   DR  Accounts Payable           amount
//   CR  Cash / Bank / UPI / Card   amount

func (r *Recorder) RecordSupplierPayment(supplierPaymentID, userID string) error {
	exists, err := r.store.VoucherExistsForRef(RefSupplierPayment, supplierPaymentID)
	if err != nil {
		return fmt.Errorf("check existing voucher: %w", err)
	}
	if exists {
		return nil
	}

	var sp struct {
		ID, InvoiceID, Method string
		Amount                float64
		PaidAt                string
	}
	err = r.db.QueryRow(`
		SELECT id, purchase_invoice_id, payment_method, amount, paid_at::date
		FROM supplier_payments WHERE id = $1
	`, supplierPaymentID).Scan(&sp.ID, &sp.InvoiceID, &sp.Method, &sp.Amount, &sp.PaidAt)
	if err != nil {
		return fmt.Errorf("read supplier payment: %w", err)
	}

	var invoiceNumber string
	r.db.QueryRow(`SELECT invoice_number FROM purchase_invoices WHERE id = $1`, sp.InvoiceID).Scan(&invoiceNumber)

	ledgerID := PaymentLedgerMap(sp.Method)
	lines := []VoucherLine{
		{LedgerAccountID: LedgerAccountsPayable, Debit: sp.Amount, Narration: "Payable settled"},
		{LedgerAccountID: ledgerID, Credit: sp.Amount, Narration: sp.Method + " paid to supplier"},
	}

	return r.store.CreateVoucher(Voucher{
		VoucherType: VoucherTypePayment,
		VoucherDate: sp.PaidAt,
		Narration:   "Payment to supplier for " + invoiceNumber,
		RefType:     RefSupplierPayment,
		RefID:       supplierPaymentID,
		CreatedBy:   userID,
		Lines:       lines,
	})
}

// ════════════════════════════════════════════
// Backfill — one-time recording of all historical data
// ════════════════════════════════════════════

func (r *Recorder) BackfillSalesInvoices(userID string) (int, error) {
	rows, err := r.db.Query(`SELECT id FROM sales_invoices WHERE status != 'CANCELLED' ORDER BY created_at`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if err := r.RecordSalesInvoice(id, userID); err != nil {
			return count, fmt.Errorf("sales invoice %s: %w", id, err)
		}
		count++
	}
	return count, nil
}

func (r *Recorder) BackfillPurchaseInvoices(userID string) (int, error) {
	rows, err := r.db.Query(`SELECT id FROM purchase_invoices WHERE status != 'CANCELLED' ORDER BY created_at`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		if err := r.RecordPurchaseInvoice(id, userID); err != nil {
			return count, fmt.Errorf("purchase invoice %s: %w", id, err)
		}
		count++
	}
	return count, nil
}

func (r *Recorder) BackfillAll(userID string) (map[string]int, error) {
	result := map[string]int{}

	n, err := r.BackfillSalesInvoices(userID)
	result["sales_invoices"] = n
	if err != nil {
		return result, err
	}

	n, err = r.BackfillPurchaseInvoices(userID)
	result["purchase_invoices"] = n
	if err != nil {
		return result, err
	}

	patched, err := r.PatchPurchaseBranchIDs()
	result["purchase_branch_patched"] = patched
	if err != nil {
		return result, err
	}

	return result, nil
}

// PatchPurchaseBranchIDs back-fills branch_id on existing purchase vouchers
// by looking up purchase_invoices → warehouses → branch_id.
func (r *Recorder) PatchPurchaseBranchIDs() (int, error) {
	res, err := r.db.Exec(`
		UPDATE vouchers v
		SET branch_id = w.branch_id
		FROM purchase_invoices pi
		JOIN warehouses w ON w.id = pi.warehouse_id
		WHERE v.ref_type = 'purchase_invoice'
		  AND v.ref_id = pi.id
		  AND v.branch_id IS NULL
		  AND w.branch_id IS NOT NULL
	`)
	if err != nil {
		return 0, fmt.Errorf("patch purchase branch ids: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
