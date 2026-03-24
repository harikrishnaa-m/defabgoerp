package purchaseinvoice

import (
	"database/sql"
	"errors"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) generateInvoiceNumber() (string, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM purchase_invoices`).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("PI-%03d", count+1), nil
}

// Create creates a purchase invoice from a PO.
func (s *Store) Create(in CreatePurchaseInvoiceInput, userID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Fetch supplier_id and warehouse_id from PO
	var supplierID, warehouseID string
	err = tx.QueryRow(`SELECT supplier_id, warehouse_id FROM purchase_orders WHERE id = $1`, in.PurchaseOrderID).Scan(&supplierID, &warehouseID)
	if err != nil {
		return "", fmt.Errorf("purchase order not found: %w", err)
	}

	// Auto-generate invoice number if not provided
	invoiceNumber := in.InvoiceNumber
	if invoiceNumber == "" {
		invoiceNumber, err = s.generateInvoiceNumber()
		if err != nil {
			return "", err
		}
	}

	// Fetch ALL items from PO and calculate totals
	rows, err := tx.Query(`
		SELECT id, item_name, COALESCE(hsn_code,''), unit, quantity, unit_price, gst_percent
		FROM purchase_order_items WHERE purchase_order_id = $1
	`, in.PurchaseOrderID)
	if err != nil {
		return "", fmt.Errorf("fetch po items: %w", err)
	}
	defer rows.Close()

	type itemCalc struct {
		poItemID   string
		itemName   string
		hsnCode    string
		unit       string
		qty        float64
		unitPrice  float64
		taxPercent float64
		taxAmount  float64
		total      float64
	}
	var items []itemCalc
	var subAmount, totalGST float64

	for rows.Next() {
		var it itemCalc
		if err := rows.Scan(&it.poItemID, &it.itemName, &it.hsnCode, &it.unit, &it.qty, &it.unitPrice, &it.taxPercent); err != nil {
			return "", err
		}
		lineTotal := it.qty * it.unitPrice
		it.taxAmount = lineTotal * it.taxPercent / 100
		it.total = lineTotal + it.taxAmount
		subAmount += lineTotal
		totalGST += it.taxAmount
		items = append(items, it)
	}

	if len(items) == 0 {
		return "", errors.New("no items found in purchase order")
	}

	netAmount := subAmount + totalGST - in.DiscountAmount + in.RoundOff

	// Insert purchase invoice
	var invoiceID string
	err = tx.QueryRow(`
		INSERT INTO purchase_invoices
			(invoice_number, purchase_order_id, supplier_id, warehouse_id,
			 invoice_date, sub_amount, discount_amount, gst_amount, round_off,
			 net_amount, paid_amount, status, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 0, 'PENDING', $11, $12)
		RETURNING id
	`, invoiceNumber, in.PurchaseOrderID, supplierID, warehouseID,
		in.InvoiceDate, subAmount, in.DiscountAmount, totalGST, in.RoundOff,
		netAmount, in.Notes, userID).Scan(&invoiceID)
	if err != nil {
		return "", fmt.Errorf("insert purchase_invoices: %w", err)
	}

	// Insert items
	for _, it := range items {
		_, err := tx.Exec(`
			INSERT INTO purchase_invoice_items
				(purchase_invoice_id, purchase_order_item_id, item_name, hsn_code,
				 unit, quantity, unit_price, tax_percent, tax_amount, total_amount)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, invoiceID, it.poItemID, it.itemName, it.hsnCode,
			it.unit, it.qty, it.unitPrice, it.taxPercent, it.taxAmount, it.total)
		if err != nil {
			return "", fmt.Errorf("insert purchase_invoice_items: %w", err)
		}
	}

	// Record payment if provided
	if in.PaymentAmount > 0 && in.PaymentMethod != "" {
		if in.PaymentAmount > netAmount {
			return "", fmt.Errorf("payment of %.2f exceeds invoice amount of %.2f", in.PaymentAmount, netAmount)
		}

		_, err = tx.Exec(`
			INSERT INTO supplier_payments (purchase_invoice_id, amount, payment_method, reference, paid_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, invoiceID, in.PaymentAmount, in.PaymentMethod, in.Reference)
		if err != nil {
			return "", fmt.Errorf("insert payment: %w", err)
		}

		status := "PARTIALLY_PAID"
		if in.PaymentAmount >= netAmount {
			status = "PAID"
		}

		_, err = tx.Exec(`
			UPDATE purchase_invoices SET paid_amount = $1, status = $2 WHERE id = $3
		`, in.PaymentAmount, status, invoiceID)
		if err != nil {
			return "", fmt.Errorf("update invoice payment: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return invoiceID, nil
}

// GetByID returns a purchase invoice with items and payments.
func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var invoiceID, invoiceNumber, poID, poNumber, supplierID, supplierName string
	var warehouseID, warehouseName, invoiceDate, status, createdAt string
	var notes, createdByName sql.NullString
	var subAmount, discountAmount, gstAmount, roundOff, netAmount, paidAmount float64

	err := s.db.QueryRow(`
		SELECT pi.id, pi.invoice_number, pi.purchase_order_id, po.po_number,
		       pi.supplier_id, s.name AS supplier_name,
		       pi.warehouse_id, w.name AS warehouse_name,
		       pi.invoice_date::text, pi.sub_amount, pi.discount_amount,
		       pi.gst_amount, pi.round_off, pi.net_amount, pi.paid_amount,
		       pi.status, pi.notes, u.name AS created_by_name, pi.created_at::text
		FROM purchase_invoices pi
		JOIN purchase_orders po ON po.id = pi.purchase_order_id
		JOIN suppliers s ON s.id = pi.supplier_id
		JOIN warehouses w ON w.id = pi.warehouse_id
		LEFT JOIN users u ON u.id = pi.created_by
		WHERE pi.id = $1
	`, id).Scan(
		&invoiceID, &invoiceNumber, &poID, &poNumber,
		&supplierID, &supplierName,
		&warehouseID, &warehouseName,
		&invoiceDate, &subAmount, &discountAmount,
		&gstAmount, &roundOff, &netAmount, &paidAmount,
		&status, &notes, &createdByName, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":                invoiceID,
		"invoice_number":    invoiceNumber,
		"purchase_order_id": poID,
		"po_number":         poNumber,
		"supplier_id":       supplierID,
		"supplier_name":     supplierName,
		"warehouse_id":      warehouseID,
		"warehouse_name":    warehouseName,
		"invoice_date":      invoiceDate,
		"sub_amount":        subAmount,
		"discount_amount":   discountAmount,
		"gst_amount":        gstAmount,
		"round_off":         roundOff,
		"net_amount":        netAmount,
		"paid_amount":       paidAmount,
		"balance_due":       netAmount - paidAmount,
		"status":            status,
		"created_at":        createdAt,
	}
	if notes.Valid {
		result["notes"] = notes.String
	}
	if createdByName.Valid {
		result["created_by_name"] = createdByName.String
	}

	// Fetch items
	itemRows, err := s.db.Query(`
		SELECT pii.id, pii.purchase_order_item_id, pii.item_name, pii.hsn_code,
		       pii.unit, pii.quantity, pii.unit_price, pii.tax_percent,
		       pii.tax_amount, pii.total_amount
		FROM purchase_invoice_items pii
		WHERE pii.purchase_invoice_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	var items []map[string]interface{}
	for itemRows.Next() {
		var iID, poItemID, iName string
		var hsnCode, unit sql.NullString
		var qty, unitPrice, taxPct, taxAmt, totalAmt float64
		if err := itemRows.Scan(&iID, &poItemID, &iName, &hsnCode,
			&unit, &qty, &unitPrice, &taxPct, &taxAmt, &totalAmt); err != nil {
			return nil, err
		}
		item := map[string]interface{}{
			"id":                     iID,
			"purchase_order_item_id": poItemID,
			"item_name":              iName,
			"quantity":               qty,
			"unit_price":             unitPrice,
			"tax_percent":            taxPct,
			"tax_amount":             taxAmt,
			"total_amount":           totalAmt,
		}
		if hsnCode.Valid {
			item["hsn_code"] = hsnCode.String
		}
		if unit.Valid {
			item["unit"] = unit.String
		}
		items = append(items, item)
	}
	result["items"] = items

	// Fetch payments
	payRows, err := s.db.Query(`
		SELECT id, amount, payment_method, reference, paid_at::text, created_at::text
		FROM supplier_payments
		WHERE purchase_invoice_id = $1
		ORDER BY paid_at
	`, id)
	if err != nil {
		return nil, err
	}
	defer payRows.Close()

	var payments []map[string]interface{}
	for payRows.Next() {
		var pID, pMethod, pPaidAt, pCreatedAt string
		var pRef sql.NullString
		var pAmount float64
		if err := payRows.Scan(&pID, &pAmount, &pMethod, &pRef, &pPaidAt, &pCreatedAt); err != nil {
			return nil, err
		}
		pay := map[string]interface{}{
			"id":             pID,
			"amount":         pAmount,
			"payment_method": pMethod,
			"paid_at":        pPaidAt,
			"created_at":     pCreatedAt,
		}
		if pRef.Valid {
			pay["reference"] = pRef.String
		}
		payments = append(payments, pay)
	}
	result["payments"] = payments

	return result, nil
}

// List returns all purchase invoices.
func (s *Store) List() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT pi.id, pi.invoice_number, pi.purchase_order_id, po.po_number,
		       pi.supplier_id, s.name AS supplier_name,
		       pi.warehouse_id, w.name AS warehouse_name,
		       pi.invoice_date::text, pi.net_amount, pi.paid_amount,
		       pi.status, pi.created_at::text
		FROM purchase_invoices pi
		JOIN purchase_orders po ON po.id = pi.purchase_order_id
		JOIN suppliers s ON s.id = pi.supplier_id
		JOIN warehouses w ON w.id = pi.warehouse_id
		ORDER BY pi.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, invNum, poID, poNum, supID, supName string
		var whID, whName, invDate, status, createdAt string
		var netAmount, paidAmount float64
		if err := rows.Scan(
			&id, &invNum, &poID, &poNum,
			&supID, &supName, &whID, &whName,
			&invDate, &netAmount, &paidAmount,
			&status, &createdAt,
		); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":                id,
			"invoice_number":    invNum,
			"purchase_order_id": poID,
			"po_number":         poNum,
			"supplier_id":       supID,
			"supplier_name":     supName,
			"warehouse_id":      whID,
			"warehouse_name":    whName,
			"invoice_date":      invDate,
			"net_amount":        netAmount,
			"paid_amount":       paidAmount,
			"balance_due":       netAmount - paidAmount,
			"status":            status,
			"created_at":        createdAt,
		})
	}
	return results, nil
}

// RecordPayment adds a payment to a purchase invoice and updates paid_amount/status.
func (s *Store) RecordPayment(invoiceID string, in RecordPaymentInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Fetch current amounts
	var netAmount, paidAmount float64
	var status string
	err = tx.QueryRow(`SELECT net_amount, paid_amount, status FROM purchase_invoices WHERE id = $1`, invoiceID).Scan(&netAmount, &paidAmount, &status)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}
	if status == "CANCELLED" {
		return errors.New("cannot pay a cancelled invoice")
	}

	newPaid := paidAmount + in.Amount
	if newPaid > netAmount {
		return fmt.Errorf("payment of %.2f exceeds balance due of %.2f", in.Amount, netAmount-paidAmount)
	}

	// Determine paid_at
	paidAt := "NOW()"
	args := []interface{}{invoiceID, in.Amount, in.PaymentMethod, in.Reference}
	if in.PaidAt != "" {
		paidAt = "$5"
		args = append(args, in.PaidAt)
	}

	// Insert payment
	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO supplier_payments (purchase_invoice_id, amount, payment_method, reference, paid_at)
		VALUES ($1, $2, $3, $4, %s)
	`, paidAt), args...)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}

	// Update invoice paid_amount and status
	newStatus := "PARTIALLY_PAID"
	if newPaid >= netAmount {
		newStatus = "PAID"
	}

	_, err = tx.Exec(`
		UPDATE purchase_invoices
		SET paid_amount = $1, status = $2
		WHERE id = $3
	`, newPaid, newStatus, invoiceID)
	if err != nil {
		return fmt.Errorf("update invoice: %w", err)
	}

	return tx.Commit()
}

// ListAllPayments returns all supplier payments with invoice and supplier details.
func (s *Store) ListAllPayments() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT sp.id, sp.amount, sp.payment_method, sp.reference,
		       sp.paid_at::text, sp.created_at::text,
		       pi.id AS invoice_id, pi.invoice_number,
		       po.po_number, s.id AS supplier_id, s.name AS supplier_name
		FROM supplier_payments sp
		JOIN purchase_invoices pi ON pi.id = sp.purchase_invoice_id
		JOIN purchase_orders po ON po.id = pi.purchase_order_id
		JOIN suppliers s ON s.id = pi.supplier_id
		ORDER BY sp.paid_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var pID, pMethod, pPaidAt, pCreatedAt string
		var invID, invNum, poNum, supID, supName string
		var pRef sql.NullString
		var pAmount float64
		if err := rows.Scan(&pID, &pAmount, &pMethod, &pRef,
			&pPaidAt, &pCreatedAt,
			&invID, &invNum, &poNum, &supID, &supName); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"id":             pID,
			"amount":         pAmount,
			"payment_method": pMethod,
			"paid_at":        pPaidAt,
			"created_at":     pCreatedAt,
			"invoice_id":     invID,
			"invoice_number": invNum,
			"po_number":      poNum,
			"supplier_id":    supID,
			"supplier_name":  supName,
		}
		if pRef.Valid {
			row["reference"] = pRef.String
		}
		results = append(results, row)
	}
	return results, nil
}

// ListPaymentsBySupplier returns all payments for a specific supplier.
func (s *Store) ListPaymentsBySupplier(supplierID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT sp.id, sp.amount, sp.payment_method, sp.reference,
		       sp.paid_at::text, sp.created_at::text,
		       pi.id AS invoice_id, pi.invoice_number, po.po_number
		FROM supplier_payments sp
		JOIN purchase_invoices pi ON pi.id = sp.purchase_invoice_id
		JOIN purchase_orders po ON po.id = pi.purchase_order_id
		WHERE pi.supplier_id = $1
		ORDER BY sp.paid_at DESC
	`, supplierID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var pID, pMethod, pPaidAt, pCreatedAt string
		var invID, invNum, poNum string
		var pRef sql.NullString
		var pAmount float64
		if err := rows.Scan(&pID, &pAmount, &pMethod, &pRef,
			&pPaidAt, &pCreatedAt, &invID, &invNum, &poNum); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"id":             pID,
			"amount":         pAmount,
			"payment_method": pMethod,
			"paid_at":        pPaidAt,
			"created_at":     pCreatedAt,
			"invoice_id":     invID,
			"invoice_number": invNum,
			"po_number":      poNum,
		}
		if pRef.Valid {
			row["reference"] = pRef.String
		}
		results = append(results, row)
	}
	return results, nil
}

// OutstandingSummary returns per-supplier totals: invoiced, paid, balance.
func (s *Store) OutstandingSummary() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT pi.supplier_id, s.name AS supplier_name,
		       COUNT(pi.id) AS total_invoices,
		       COALESCE(SUM(pi.net_amount), 0) AS total_invoiced,
		       COALESCE(SUM(pi.paid_amount), 0) AS total_paid,
		       COALESCE(SUM(pi.net_amount - pi.paid_amount), 0) AS balance_due
		FROM purchase_invoices pi
		JOIN suppliers s ON s.id = pi.supplier_id
		WHERE pi.status != 'CANCELLED'
		GROUP BY pi.supplier_id, s.name
		ORDER BY balance_due DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var supID, supName string
		var totalInvoices int
		var totalInvoiced, totalPaid, balanceDue float64
		if err := rows.Scan(&supID, &supName, &totalInvoices, &totalInvoiced, &totalPaid, &balanceDue); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"supplier_id":    supID,
			"supplier_name":  supName,
			"total_invoices": totalInvoices,
			"total_invoiced": totalInvoiced,
			"total_paid":     totalPaid,
			"balance_due":    balanceDue,
		})
	}
	return results, nil
}

// Cancel marks a PENDING invoice as CANCELLED.
func (s *Store) Cancel(invoiceID string) error {
	var status string
	err := s.db.QueryRow(`SELECT status FROM purchase_invoices WHERE id = $1`, invoiceID).Scan(&status)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}
	if status != "PENDING" {
		return fmt.Errorf("only PENDING invoices can be cancelled, current status: %s", status)
	}

	_, err = s.db.Exec(`UPDATE purchase_invoices SET status = 'CANCELLED' WHERE id = $1`, invoiceID)
	return err
}
