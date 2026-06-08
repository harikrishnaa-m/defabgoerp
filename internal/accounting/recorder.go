package accounting

import (
	"database/sql"
	"fmt"
	"math"
	"time"
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

	// Exchange-generated invoices are accounted for by RecordExchange — skip them.
	var channel string
	r.db.QueryRow(`SELECT channel FROM sales_invoices WHERE id = $1`, salesInvoiceID).Scan(&channel)
	if channel == "EXCHANGE" {
		return nil
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

	// sub_amount is the full MRP (GST-inclusive); revenue = sub_amount − gst_amount
	salesRevenue := math.Round((inv.SubAmount-inv.GSTAmount)*100) / 100

	lines := []VoucherLine{
		{LedgerAccountID: LedgerAccountsReceiv, Debit: inv.NetAmount, Narration: "Customer receivable"},
		{LedgerAccountID: LedgerSalesRevenue, Credit: salesRevenue, Narration: "Sales revenue (excl. GST)"},
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

func (r *Recorder) RecordSalesReturn(returnOrderID, userID string) error {
	exists, err := r.store.VoucherExistsForRef(RefSalesReturn, returnOrderID)
	if err != nil {
		return fmt.Errorf("check existing voucher: %w", err)
	}
	if exists {
		return nil
	}

	// Exchange-generated credit notes are accounted for by RecordExchange — skip them.
	var source string
	r.db.QueryRow(`SELECT source FROM return_orders WHERE id = $1`, returnOrderID).Scan(&source)
	if source == "EXCHANGE" {
		return nil
	}

	var ret struct {
		ID             string
		ReturnNumber   string
		SalesInvoiceID string
		BranchID       sql.NullString
		TotalAmount    float64
		GSTAmount      float64
		RefundType     sql.NullString
		RefundMethod   sql.NullString
	}
	err = r.db.QueryRow(`
		SELECT id, return_number, sales_invoice_id, branch_id,
		       total_amount, gst_amount, refund_type, refund_method
		FROM return_orders WHERE id = $1
	`, returnOrderID).Scan(&ret.ID, &ret.ReturnNumber, &ret.SalesInvoiceID,
		&ret.BranchID, &ret.TotalAmount, &ret.GSTAmount, &ret.RefundType, &ret.RefundMethod)
	if err != nil {
		return fmt.Errorf("read return order: %w", err)
	}

	revenueReversal := ret.TotalAmount - ret.GSTAmount
	if revenueReversal < 0 {
		revenueReversal = 0
	}

	lines := []VoucherLine{
		{LedgerAccountID: LedgerSalesRevenue, Debit: revenueReversal, Narration: "Sales return revenue reversal"},
	}
	if ret.GSTAmount > 0 {
		lines = append(lines, VoucherLine{LedgerAccountID: LedgerGSTPayable, Debit: ret.GSTAmount, Narration: "Sales return GST reversal"})
	}

	refundType := "CASH"
	if ret.RefundType.Valid && ret.RefundType.String == "CREDIT" {
		refundType = "CREDIT"
	}

	switch refundType {
	case "CREDIT":
		lines = append(lines, VoucherLine{
			LedgerAccountID: LedgerAccountsReceiv,
			Credit:          ret.TotalAmount,
			Narration:       "Customer credit for sales return",
		})
	default:
		paymentLedger := LedgerCash
		if ret.RefundMethod.Valid {
			paymentLedger = PaymentLedgerMap(ret.RefundMethod.String)
		}
		lines = append(lines, VoucherLine{
			LedgerAccountID: paymentLedger,
			Credit:          ret.TotalAmount,
			Narration:       "Cash refund for sales return",
		})
	}

	branchID := ""
	if ret.BranchID.Valid {
		branchID = ret.BranchID.String
	}

	return r.store.CreateVoucher(Voucher{
		VoucherType: VoucherTypeSalesReturn,
		VoucherDate: time.Now().Format("2006-01-02"),
		Narration:   "Sales Return " + ret.ReturnNumber,
		RefType:     RefSalesReturn,
		RefID:       returnOrderID,
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

	n, err = r.BackfillSalesReturns(userID)
	result["sales_returns"] = n
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

func (r *Recorder) BackfillSalesReturns(userID string) (int, error) {
	rows, err := r.db.Query(`SELECT id FROM return_orders WHERE status != 'CANCELLED' ORDER BY created_at`)
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
		if err := r.RecordSalesReturn(id, userID); err != nil {
			return count, fmt.Errorf("sales return %s: %w", id, err)
		}
		count++
	}
	return count, nil
}

// CancelVoucherByRef cancels the voucher linked to a specific ref_type + ref_id.
func (r *Recorder) CancelVoucherByRef(refType, refID string) error {
	return r.store.CancelVoucherByRef(refType, refID)
}

// ════════════════════════════════════════════
// Exchange Order → EXCHANGE (JOURNAL) voucher
// ════════════════════════════════════════════
//
// A single net journal that captures both sides of the exchange:
//
//   DR  Sales Returns A/c         items_out (excl GST)   ← revenue reversal for returned items
//   DR  GST Payable               items_out GST          ← output GST reversal
//   CR  Sales Revenue A/c         items_in  (excl GST)   ← revenue for new items
//   CR  GST Payable               items_in  GST          ← output GST on new items
//
//   If customer pays net difference (COLLECT):
//   DR  Cash / Card / UPI         net_amount
//
//   If store refunds net difference (REFUND):
//   CR  Cash / Card / UPI         |net_amount|
//
// The credit note (return_order, source='EXCHANGE') and the new sales invoice
// (channel='EXCHANGE') exist in the DB for GST compliance but are NOT recorded
// separately — RecordSalesReturn and RecordSalesInvoice skip them.

func (r *Recorder) RecordExchange(exchangeOrderID, userID string) error {
	exists, err := r.store.VoucherExistsForRef(RefExchange, exchangeOrderID)
	if err != nil {
		return fmt.Errorf("check existing exchange voucher: %w", err)
	}
	if exists {
		return nil // idempotent
	}

	var exc struct {
		ExchangeNumber string
		BranchID       sql.NullString
		ItemsOutTotal  float64
		ItemsInTotal   float64
		NetAmount      float64
		CreatedAt      string
	}
	if err := r.db.QueryRow(`
		SELECT exchange_number, branch_id,
		       items_out_total, items_in_total, net_amount,
		       created_at::date
		FROM exchange_orders WHERE id = $1
	`, exchangeOrderID).Scan(
		&exc.ExchangeNumber, &exc.BranchID,
		&exc.ItemsOutTotal, &exc.ItemsInTotal, &exc.NetAmount,
		&exc.CreatedAt,
	); err != nil {
		return fmt.Errorf("read exchange order: %w", err)
	}

	// Aggregate GST from the two sides
	var gstOut, gstIn float64
	r.db.QueryRow(`SELECT COALESCE(SUM(tax_amount),0) FROM exchange_items_out WHERE exchange_order_id = $1`, exchangeOrderID).Scan(&gstOut)
	r.db.QueryRow(`SELECT COALESCE(SUM(tax_amount),0) FROM exchange_items_in  WHERE exchange_order_id = $1`, exchangeOrderID).Scan(&gstIn)

	gstOut = math.Round(gstOut*100) / 100
	gstIn = math.Round(gstIn*100) / 100
	revenueOut := math.Round((exc.ItemsOutTotal-gstOut)*100) / 100
	revenueIn := math.Round((exc.ItemsInTotal-gstIn)*100) / 100
	if revenueOut < 0 {
		revenueOut = 0
	}
	if revenueIn < 0 {
		revenueIn = 0
	}

	lines := []VoucherLine{
		{LedgerAccountID: LedgerSalesRevenue, Debit: revenueOut, Narration: "Exchange: revenue reversal for returned items"},
	}
	if gstOut > 0 {
		lines = append(lines, VoucherLine{
			LedgerAccountID: LedgerGSTPayable, Debit: gstOut, Narration: "Exchange: GST reversal on returned items",
		})
	}
	lines = append(lines, VoucherLine{
		LedgerAccountID: LedgerSalesRevenue, Credit: revenueIn, Narration: "Exchange: revenue for new items",
	})
	if gstIn > 0 {
		lines = append(lines, VoucherLine{
			LedgerAccountID: LedgerGSTPayable, Credit: gstIn, Narration: "Exchange: GST on new items",
		})
	}

	// Settlement cash movements
	if exc.NetAmount != 0 {
		rows, err := r.db.Query(`
			SELECT payment_method, SUM(amount), direction
			FROM exchange_settlements
			WHERE exchange_order_id = $1
			GROUP BY payment_method, direction
		`, exchangeOrderID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var method, direction string
				var amt float64
				if err := rows.Scan(&method, &amt, &direction); err != nil {
					continue
				}
				ledgerID := PaymentLedgerMap(method)
				if direction == "COLLECT" {
					lines = append(lines, VoucherLine{
						LedgerAccountID: ledgerID, Debit: amt,
						Narration: "Exchange: cash collected from customer (" + method + ")",
					})
				} else {
					lines = append(lines, VoucherLine{
						LedgerAccountID: ledgerID, Credit: amt,
						Narration: "Exchange: cash refunded to customer (" + method + ")",
					})
				}
			}
		}
	}

	branchID := ""
	if exc.BranchID.Valid {
		branchID = exc.BranchID.String
	}

	_ = time.Now() // satisfy import
	return r.store.CreateVoucher(Voucher{
		VoucherType: VoucherTypeExchange,
		VoucherDate: exc.CreatedAt,
		Narration:   "Exchange " + exc.ExchangeNumber,
		RefType:     RefExchange,
		RefID:       exchangeOrderID,
		BranchID:    branchID,
		CreatedBy:   userID,
		Lines:       lines,
	})
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
