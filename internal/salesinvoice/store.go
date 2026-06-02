package salesinvoice

import (
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) List(branchID *string, status, search string, limit, offset int) ([]map[string]interface{}, int, error) {
	baseWhere := "WHERE 1=1"
	var args []interface{}
	argIdx := 1

	if branchID != nil {
		baseWhere += fmt.Sprintf(" AND si.branch_id = $%d", argIdx)
		args = append(args, *branchID)
		argIdx++
	}

	if status != "" {
		baseWhere += fmt.Sprintf(" AND si.status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	if search != "" {
		baseWhere += fmt.Sprintf(" AND (si.invoice_number ILIKE $%d OR so.so_number ILIKE $%d OR c.name ILIKE $%d OR c.phone ILIKE $%d)", argIdx, argIdx, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	// Count
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		LEFT JOIN customers c ON c.id = si.customer_id
		%s`, baseWhere)
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List
	query := fmt.Sprintf(`
		SELECT
			si.id, si.invoice_number, si.channel, si.invoice_date,
			si.sub_amount, si.discount_amount, si.bill_discount, si.gst_amount,
			si.round_off, si.net_amount, si.paid_amount, si.status,
			si.created_at,
			so.so_number,
			COALESCE(c.name, '') AS customer_name,
			COALESCE(c.phone, '') AS customer_phone,
			COALESCE(b.name, '') AS branch_name,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(w.name, '') AS warehouse_name,
			COALESCE(ro.return_number, '') AS return_number
		FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		LEFT JOIN customers c ON c.id = si.customer_id
		LEFT JOIN branches b ON b.id = si.branch_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
		LEFT JOIN warehouses w ON w.id = si.warehouse_id
		LEFT JOIN return_orders ro ON ro.id = si.return_order_id
		%s
		ORDER BY si.created_at DESC
	`, baseWhere)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, limit, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		var id, invNumber, channel, status, soNumber string
		var customerName, customerPhone, branchName, spName, whName string
		var returnNumber string
		var invoiceDate, createdAt interface{}
		var subAmount, discountAmount, billDiscount, gstAmount, roundOff, netAmount, paidAmount float64

		if err := rows.Scan(
			&id, &invNumber, &channel, &invoiceDate,
			&subAmount, &discountAmount, &billDiscount, &gstAmount,
			&roundOff, &netAmount, &paidAmount, &status,
			&createdAt,
			&soNumber,
			&customerName, &customerPhone,
			&branchName, &spName, &whName,
			&returnNumber,
		); err != nil {
			return nil, 0, err
		}

		var returnNumVal interface{}
		if returnNumber != "" {
			returnNumVal = returnNumber
		}

		out = append(out, map[string]interface{}{
			"id":               id,
			"invoice_number":   invNumber,
			"so_number":        soNumber,
			"channel":          channel,
			"invoice_date":     invoiceDate,
			"sub_amount":       subAmount,
			"discount_amount":  discountAmount,
			"bill_discount":    billDiscount,
			"gst_amount":       gstAmount,
			"cgst":             gstAmount / 2,
			"sgst":             gstAmount / 2,
			"round_off":        roundOff,
			"net_amount":       netAmount,
			"paid_amount":      paidAmount,
			"balance_due":      netAmount - paidAmount,
			"status":           status,
			"customer_name":    customerName,
			"customer_phone":   customerPhone,
			"branch_name":      branchName,
			"salesperson_name": spName,
			"warehouse_name":   whName,
			"created_at":       createdAt,
			"return_number":    returnNumVal,
		})
	}

	return out, total, nil
}

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var invID, invNumber, channel, status, soNumber, soID string
	var customerID, customerCode, customerName, customerPhone, customerEmail string
	var branchName, spName, whName, createdByName string
	var spID sql.NullString
	var returnNumber string
	var invoiceDate, createdAt, updatedAt interface{}
	var subAmount, discountAmount, billDiscount, gstAmount, roundOff, netAmount, paidAmount float64

	err := s.db.QueryRow(`
		SELECT
			si.id, si.invoice_number, si.channel, si.invoice_date,
			si.sub_amount, si.discount_amount, si.bill_discount, si.gst_amount,
			si.round_off, si.net_amount, si.paid_amount, si.status,
			si.created_at, si.updated_at,
			so.id, so.so_number,
			c.id, COALESCE(c.customer_code, ''), c.name, COALESCE(c.phone, ''), COALESCE(c.email, ''),
			COALESCE(b.name, '') AS branch_name,
			so.salesperson_id,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(w.name, '') AS warehouse_name,
			COALESCE(u.name, '') AS created_by_name,
			COALESCE(ro.return_number, '') AS return_number
		FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		JOIN customers c ON c.id = si.customer_id
		LEFT JOIN branches b ON b.id = si.branch_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
		LEFT JOIN warehouses w ON w.id = si.warehouse_id
		LEFT JOIN users u ON u.id = si.created_by
		LEFT JOIN return_orders ro ON ro.id = si.return_order_id
		WHERE si.id = $1
	`, id).Scan(
		&invID, &invNumber, &channel, &invoiceDate,
		&subAmount, &discountAmount, &billDiscount, &gstAmount,
		&roundOff, &netAmount, &paidAmount, &status,
		&createdAt, &updatedAt,
		&soID, &soNumber,
		&customerID, &customerCode, &customerName, &customerPhone, &customerEmail,
		&branchName, &spID, &spName, &whName, &createdByName,
		&returnNumber,
	)
	if err != nil {
		return nil, err
	}

	var returnNumVal interface{}
	if returnNumber != "" {
		returnNumVal = returnNumber
	}

	invoice := map[string]interface{}{
		"id":              invID,
		"invoice_number":  invNumber,
		"channel":         channel,
		"invoice_date":    invoiceDate,
		"sub_amount":      subAmount,
		"discount_amount": discountAmount,
		"bill_discount":   billDiscount,
		"gst_amount":      gstAmount,
		"cgst":            gstAmount / 2,
		"sgst":            gstAmount / 2,
		"round_off":       roundOff,
		"net_amount":      netAmount,
		"paid_amount":     paidAmount,
		"balance_due":     netAmount - paidAmount,
		"status":          status,
		"created_by":      createdByName,
		"created_at":      createdAt,
		"updated_at":      updatedAt,
		"return_number":   returnNumVal,
		"sales_order": map[string]interface{}{
			"id":        soID,
			"so_number": soNumber,
		},
		"customer": map[string]interface{}{
			"id":            customerID,
			"customer_code": customerCode,
			"name":          customerName,
			"phone":         customerPhone,
			"email":         customerEmail,
		},
		"branch_name":      branchName,
		"salesperson_id":   spID.String,
		"salesperson_name": spName,
		"warehouse_name":   whName,
	}

	// Invoice items with product/variant details
	itemRows, err := s.db.Query(`
		SELECT
			sii.id, sii.quantity, sii.unit_price, sii.discount,
			sii.tax_percent, sii.tax_amount, sii.total_price,
			COALESCE(sii.item_description, ''),
			COALESCE(v.variant_code, 0), COALESCE(v.sku, ''),
			COALESCE(v.name, '') AS variant_name,
			COALESCE(v.barcode, '') AS barcode,
			COALESCE(p.name, '') AS product_name
		FROM sales_invoice_items sii
		LEFT JOIN variants v ON v.id = sii.variant_id
		LEFT JOIN products p ON p.id = v.product_id
		WHERE sii.sales_invoice_id = $1
		ORDER BY sii.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	var items []map[string]interface{}
	for itemRows.Next() {
		var itemID, sku, variantName, barcode, productName, itemDescription string
		var variantCode int
		var quantity float64
		var unitPrice, discount, taxPercent, taxAmount, totalPrice float64

		if err := itemRows.Scan(
			&itemID, &quantity, &unitPrice, &discount,
			&taxPercent, &taxAmount, &totalPrice,
			&itemDescription,
			&variantCode, &sku, &variantName, &barcode, &productName,
		); err != nil {
			return nil, err
		}

		items = append(items, map[string]interface{}{
			"id":               itemID,
			"item_description": itemDescription,
			"product_name":     productName,
			"variant_name":     variantName,
			"variant_code":     variantCode,
			"sku":              sku,
			"barcode":          barcode,
			"quantity":         quantity,
			"unit_price":       unitPrice,
			"discount":         discount,
			"tax_percent":      taxPercent,
			"cgst_percent":     taxPercent / 2,
			"sgst_percent":     taxPercent / 2,
			"tax_amount":       taxAmount,
			"cgst_amount":      taxAmount / 2,
			"sgst_amount":      taxAmount / 2,
			"total_price":      totalPrice,
		})
	}
	invoice["items"] = items

	// Payments
	payRows, err := s.db.Query(`
		SELECT id, amount, payment_method, COALESCE(reference, ''), paid_at
		FROM sales_payments
		WHERE sales_invoice_id = $1
		ORDER BY paid_at
	`, id)
	if err != nil {
		return nil, err
	}
	defer payRows.Close()

	var payments []map[string]interface{}
	for payRows.Next() {
		var payID, method, ref string
		var amount float64
		var paidAt interface{}
		if err := payRows.Scan(&payID, &amount, &method, &ref, &paidAt); err != nil {
			return nil, err
		}
		payments = append(payments, map[string]interface{}{
			"id":             payID,
			"amount":         amount,
			"payment_method": method,
			"reference":      ref,
			"paid_at":        paidAt,
		})
	}
	invoice["payments"] = payments

	return invoice, nil
}

func (s *Store) GetByInvoiceNumber(invoiceNumber string) (map[string]interface{}, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM sales_invoices WHERE invoice_number = $1`, invoiceNumber).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

// UpdateSalesperson changes the salesperson on the sales_order linked to the invoice.
func (s *Store) UpdateSalesperson(invoiceID, salespersonID string) error {
	var soID string
	var status string
	err := s.db.QueryRow(`SELECT sales_order_id, status FROM sales_invoices WHERE id = $1`, invoiceID).Scan(&soID, &status)
	if err != nil {
		return err
	}
	if status == "CANCELLED" {
		return fmt.Errorf("cannot update a cancelled invoice")
	}
	_, err = s.db.Exec(`UPDATE sales_orders SET salesperson_id = $1, updated_at = NOW() WHERE id = $2`, salespersonID, soID)
	return err
}

// CancelInvoice cancels a sales invoice within a single transaction.
// It restores stock, updates statuses, and reverses the customer total.
// Returns the list of sales_payment IDs so the caller can cancel their accounting vouchers.
func (s *Store) CancelInvoice(id string) (paymentIDs []string, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// ── 1. Lock + validate the invoice ──
	var status, salesOrderID, warehouseID, customerID string
	var netAmount float64
	err = tx.QueryRow(`
		SELECT status, sales_order_id, warehouse_id, customer_id, net_amount
		FROM sales_invoices WHERE id = $1 FOR UPDATE
	`, id).Scan(&status, &salesOrderID, &warehouseID, &customerID, &netAmount)
	if err != nil {
		return nil, err
	}
	if status == "CANCELLED" {
		return nil, fmt.Errorf("invoice is already cancelled")
	}
	if status == "RETURNED" {
		return nil, fmt.Errorf("cannot cancel a returned invoice")
	}

	// ── 2. Collect invoice items for stock reversal ──
	itemRows, err := tx.Query(`
		SELECT variant_id, quantity FROM sales_invoice_items WHERE sales_invoice_id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("read invoice items: %w", err)
	}
	type itemLine struct {
		variantID string
		quantity  float64
	}
	var items []itemLine
	for itemRows.Next() {
		var il itemLine
		if err := itemRows.Scan(&il.variantID, &il.quantity); err != nil {
			itemRows.Close()
			return nil, fmt.Errorf("scan invoice item: %w", err)
		}
		items = append(items, il)
	}
	itemRows.Close()

	// ── 3. Restore stock + insert CANCEL_SALE movements ──
	invoiceRef := "CANCEL_SALE:" + id
	for _, it := range items {
		_, err = tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
			VALUES ($1, $2, $3, 'PRODUCT', NOW())
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity, updated_at = NOW()
		`, it.variantID, warehouseID, it.quantity)
		if err != nil {
			return nil, fmt.Errorf("restore stock: %w", err)
		}

		_, err = tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, to_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1, $2, $3, 'CANCEL_SALE', $4, 'COMPLETED')
		`, it.variantID, warehouseID, it.quantity, invoiceRef)
		if err != nil {
			return nil, fmt.Errorf("insert cancel stock movement: %w", err)
		}
	}

	// ── 4. Mark sales_invoice CANCELLED ──
	_, err = tx.Exec(`UPDATE sales_invoices SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("cancel invoice: %w", err)
	}

	// ── 5. Mark sales_order CANCELLED ──
	_, err = tx.Exec(`UPDATE sales_orders SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1`, salesOrderID)
	if err != nil {
		return nil, fmt.Errorf("cancel sales order: %w", err)
	}

	// ── 6. Reverse customer total_purchases ──
	_, err = tx.Exec(`
		UPDATE customers SET total_purchases = GREATEST(total_purchases - $1, 0), updated_at = NOW()
		WHERE id = $2
	`, netAmount, customerID)
	if err != nil {
		return nil, fmt.Errorf("reverse customer total: %w", err)
	}

	// ── 7. Collect payment IDs for accounting voucher cancellation ──
	payRows, err := tx.Query(`SELECT id FROM sales_payments WHERE sales_invoice_id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("read payments: %w", err)
	}
	for payRows.Next() {
		var pid string
		if err := payRows.Scan(&pid); err != nil {
			payRows.Close()
			return nil, fmt.Errorf("scan payment id: %w", err)
		}
		paymentIDs = append(paymentIDs, pid)
	}
	payRows.Close()

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return paymentIDs, nil
}
