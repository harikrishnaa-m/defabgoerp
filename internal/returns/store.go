package returns

import (
	"database/sql"
	"fmt"
	"math"
)

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateReturnOrder(in CreateReturnOrderInput, userID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var invoice struct {
		ID             string
		SalesOrderID   string
		CustomerID     string
		BranchID       sql.NullString
		WarehouseID    string
		Status         string
		SubAmount      float64
		DiscountAmount float64
		BillDiscount   float64
		GSTAmount      float64
		NetAmount      float64
		PaidAmount     float64
	}
	if err := tx.QueryRow(`
		SELECT id, sales_order_id, customer_id, branch_id, warehouse_id, status,
		       sub_amount, discount_amount, bill_discount, gst_amount, net_amount, paid_amount
		FROM sales_invoices WHERE id = $1
	`, in.SalesInvoiceID).Scan(&invoice.ID, &invoice.SalesOrderID, &invoice.CustomerID,
		&invoice.BranchID, &invoice.WarehouseID, &invoice.Status, &invoice.SubAmount,
		&invoice.DiscountAmount, &invoice.BillDiscount,
		&invoice.GSTAmount, &invoice.NetAmount, &invoice.PaidAmount); err != nil {
		return "", fmt.Errorf("sales invoice lookup: %w", err)
	}
	if invoice.Status == "CANCELLED" || invoice.Status == "RETURNED" {
		return "", fmt.Errorf("cannot return against a %s invoice", invoice.Status)
	}

	type stockUpdate struct {
		variantID string
		quantity  float64
	}
	var stockUpdates []stockUpdate
	var totalLineAmount, totalItemDiscount, totalBillDiscount, totalTax, totalRefundAmount float64
	var returnID string
	if err := tx.QueryRow(`
		INSERT INTO return_orders
			(return_number, sales_invoice_id, branch_id, warehouse_id, customer_id,
			 status, refund_type, total_amount, gst_amount, created_by, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, 0, $8, $9)
		RETURNING id
	`, s.nextReturnNumber(tx), invoice.ID, invoice.BranchID, invoice.WarehouseID,
		invoice.CustomerID, ReturnStatusRequested, RefundTypeCash, userID, in.Notes).Scan(&returnID); err != nil {
		return "", fmt.Errorf("create return order: %w", err)
	}

	for _, item := range in.Items {
		if item.Quantity <= 0 {
			return "", fmt.Errorf("quantity must be greater than zero")
		}

		var siItem struct {
			ID         string
			VariantID  string
			Quantity   float64
			UnitPrice  float64
			Discount   float64
			TaxPercent float64
		}
		if err := tx.QueryRow(`
			SELECT id, variant_id, quantity, unit_price, discount, tax_percent
			FROM sales_invoice_items WHERE id = $1
		`, item.SalesInvoiceItemID).Scan(&siItem.ID, &siItem.VariantID, &siItem.Quantity,
			&siItem.UnitPrice, &siItem.Discount, &siItem.TaxPercent); err != nil {
			return "", fmt.Errorf("sales invoice item lookup: %w", err)
		}
		if item.Quantity > siItem.Quantity {
			return "", fmt.Errorf("return quantity %.2f exceeds invoiced quantity %.2f", item.Quantity, siItem.Quantity)
		}

		var alreadyReturned float64
		if err := tx.QueryRow(`
			SELECT COALESCE(SUM(quantity), 0)
			FROM return_items ri
			JOIN return_orders ro ON ri.return_order_id = ro.id
			WHERE ro.sales_invoice_id = $1 AND ri.sales_invoice_item_id = $2 AND ro.status != $3
		`, invoice.ID, siItem.ID, ReturnStatusCancelled).Scan(&alreadyReturned); err != nil {
			return "", fmt.Errorf("return quantity check: %w", err)
		}
		if alreadyReturned+item.Quantity > siItem.Quantity {
			return "", fmt.Errorf("cannot return more than invoiced quantity for item %s (already returned %.2f)", siItem.ID, alreadyReturned)
		}

		lineTotal := item.Quantity * siItem.UnitPrice
		itemDiscount := round2(siItem.Discount * item.Quantity / siItem.Quantity)
		lineBillDisc := 0.0
		if invoice.SubAmount > 0 {
			lineBillDisc = round2(lineTotal * invoice.BillDiscount / invoice.SubAmount)
		}
		taxable := lineTotal - itemDiscount - lineBillDisc
		if taxable < 0 {
			taxable = 0
		}
		lineTax := round2(taxable * siItem.TaxPercent / 100)
		lineReturnAmount := round2(lineTotal - itemDiscount - lineBillDisc + lineTax)

		_, err := tx.Exec(`
			INSERT INTO return_items
				(return_order_id, sales_invoice_item_id, variant_id, quantity, unit_price,
				 discount, bill_discount_share, tax_percent, tax_amount, total_price, reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, returnID, siItem.ID, siItem.VariantID, item.Quantity, siItem.UnitPrice,
			itemDiscount, lineBillDisc, siItem.TaxPercent, lineTax, lineReturnAmount, item.Reason)
		if err != nil {
			return "", fmt.Errorf("insert return item: %w", err)
		}

		stockUpdates = append(stockUpdates, stockUpdate{variantID: siItem.VariantID, quantity: item.Quantity})
		totalLineAmount += lineTotal
		totalItemDiscount += itemDiscount
		totalBillDiscount += lineBillDisc
		totalTax += lineTax
		totalRefundAmount += lineReturnAmount
	}

	totalRefundAmount = round2(totalRefundAmount)
	totalTax = round2(totalTax)
	if err := tx.QueryRow(`
		UPDATE return_orders
		SET total_amount = $1, gst_amount = $2
		WHERE id = $3
		RETURNING id
	`, totalRefundAmount, totalTax, returnID).Scan(&returnID); err != nil {
		return "", fmt.Errorf("update return totals: %w", err)
	}

	refundType := in.RefundType
	if refundType == "" {
		refundType = RefundTypeCash
	}
	if refundType != RefundTypeCash && refundType != RefundTypeCredit {
		return "", fmt.Errorf("invalid refund_type %s", refundType)
	}

	refundAmount := 0.0
	if refundType == RefundTypeCash {
		refundAmount = totalRefundAmount
		if in.RefundMethod == "" {
			return "", fmt.Errorf("refund_method is required for cash refunds")
		}
	}

	for _, stockUpdate := range stockUpdates {
		_, err = tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
			VALUES ($1,$2,$3,'PRODUCT', NOW())
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity, updated_at = NOW()
		`, stockUpdate.variantID, invoice.WarehouseID, stockUpdate.quantity)
		if err != nil {
			return "", err
		}
		_, err = tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, to_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1, $2, $3, 'RETURN_IN', $4, 'COMPLETED')
		`, stockUpdate.variantID, invoice.WarehouseID, stockUpdate.quantity, returnID)
		if err != nil {
			return "", err
		}
	}

	if refundAmount > 0 {
		_, err = tx.Exec(`
			INSERT INTO return_payments
				(return_order_id, amount, payment_method, reference, paid_at)
			VALUES ($1,$2,$3,$4,NOW())
		`, returnID, refundAmount, in.RefundMethod, in.RefundReference)
		if err != nil {
			return "", err
		}
	}

	invoice.SubAmount = round2(invoice.SubAmount - totalLineAmount)
	invoice.DiscountAmount = round2(invoice.DiscountAmount - totalItemDiscount)
	invoice.BillDiscount = round2(invoice.BillDiscount - totalBillDiscount)
	invoice.GSTAmount = round2(invoice.GSTAmount - totalTax)
	invoice.NetAmount = round2(invoice.NetAmount - totalRefundAmount)
	if invoice.SubAmount < 0 {
		invoice.SubAmount = 0
	}
	if invoice.DiscountAmount < 0 {
		invoice.DiscountAmount = 0
	}
	if invoice.BillDiscount < 0 {
		invoice.BillDiscount = 0
	}
	if invoice.GSTAmount < 0 {
		invoice.GSTAmount = 0
	}
	if invoice.NetAmount < 0 {
		invoice.NetAmount = 0
	}

	if refundType == RefundTypeCash {
		invoice.PaidAmount -= refundAmount
		if invoice.PaidAmount < 0 {
			invoice.PaidAmount = 0
		}
	}

	invoiceStatus := "UNPAID"
	if invoice.PaidAmount >= invoice.NetAmount && invoice.NetAmount > 0 {
		invoiceStatus = "PAID"
	} else if invoice.PaidAmount > 0 {
		invoiceStatus = "PARTIAL"
	} else if invoice.NetAmount == 0 {
		invoiceStatus = "RETURNED"
	}

	_, err = tx.Exec(`
		UPDATE sales_invoices
		SET sub_amount = $1, discount_amount = $2, bill_discount = $3,
		    gst_amount = $4, net_amount = $5, paid_amount = $6,
		    status = $7, updated_at = NOW()
		WHERE id = $8
	`, invoice.SubAmount, invoice.DiscountAmount, invoice.BillDiscount,
		invoice.GSTAmount, invoice.NetAmount, invoice.PaidAmount,
		invoiceStatus, invoice.ID)
	if err != nil {
		return "", err
	}

	orderStatus := "PARTIAL_RETURN"
	if invoice.NetAmount == 0 {
		orderStatus = "RETURNED"
	}
	_, err = tx.Exec(`
		UPDATE sales_orders SET status = $1 WHERE id = $2
	`, orderStatus, invoice.SalesOrderID)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(`
		UPDATE return_orders
		SET status = $1, refund_type = $2, refund_method = $3,
		    refund_reference = $4, refund_amount = $5, processed_at = NOW()
		WHERE id = $6
	`, ReturnStatusCompleted, refundType, in.RefundMethod, in.RefundReference, refundAmount, returnID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return returnID, nil
}

func (s *Store) List(branchID *string, status, search string, limit, offset int) ([]map[string]interface{}, int, error) {
	baseWhere := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if branchID != nil {
		baseWhere += fmt.Sprintf(" AND ro.branch_id = $%d", idx)
		args = append(args, *branchID)
		idx++
	}
	if status != "" {
		baseWhere += fmt.Sprintf(" AND ro.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if search != "" {
		baseWhere += fmt.Sprintf(" AND (ro.return_number ILIKE $%d OR si.invoice_number ILIKE $%d)", idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM return_orders ro JOIN sales_invoices si ON si.id = ro.sales_invoice_id %s`, baseWhere)
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT ro.id, ro.return_number, ro.sales_invoice_id, si.invoice_number,
		       ro.branch_id, ro.warehouse_id, ro.customer_id, ro.status,
		       ro.total_amount, ro.gst_amount, ro.refund_amount, ro.refund_type,
		       ro.refund_method, ro.refund_reference, ro.created_at, ro.processed_at, ro.notes,
		       COALESCE(b.name, '') AS branch_name,
		       COALESCE(w.name, '') AS warehouse_name,
		       COALESCE(c.name, '') AS customer_name,
		       COALESCE(c.phone, '') AS customer_phone,
		       COALESCE(c.customer_code, '') AS customer_code,
		       COALESCE(u.name, '') AS created_by_name
		FROM return_orders ro
		JOIN sales_invoices si ON si.id = ro.sales_invoice_id
		LEFT JOIN branches b ON b.id = ro.branch_id
		LEFT JOIN warehouses w ON w.id = ro.warehouse_id
		LEFT JOIN customers c ON c.id = ro.customer_id
		LEFT JOIN users u ON u.id = ro.created_by
		%s
		ORDER BY ro.created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseWhere, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []map[string]interface{}{}
	for rows.Next() {
		var roID, roNumber, salesInvoiceID, invoiceNumber, branchIDVal, warehouseID, customerID, status, refundType, refundMethod, refundReference, notes string
		var branchName, warehouseName, customerName, customerPhone, customerCode, createdByName string
		var totalAmount, gstAmount, refundAmount float64
		var createdAt, processedAt interface{}
		if err := rows.Scan(&roID, &roNumber, &salesInvoiceID, &invoiceNumber,
			&branchIDVal, &warehouseID, &customerID, &status,
			&totalAmount, &gstAmount, &refundAmount, &refundType,
			&refundMethod, &refundReference, &createdAt, &processedAt, &notes,
			&branchName, &warehouseName, &customerName, &customerPhone, &customerCode, &createdByName); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]interface{}{
			"id":               roID,
			"return_number":    roNumber,
			"sales_invoice_id": salesInvoiceID,
			"invoice_number":   invoiceNumber,
			"branch_id":        branchIDVal,
			"branch_name":      branchName,
			"warehouse_id":     warehouseID,
			"warehouse_name":   warehouseName,
			"customer_id":      customerID,
			"customer_name":    customerName,
			"customer_phone":   customerPhone,
			"customer_code":    customerCode,
			"status":           status,
			"total_amount":     totalAmount,
			"gst_amount":       gstAmount,
			"cgst":             gstAmount / 2,
			"sgst":             gstAmount / 2,
			"refund_amount":    refundAmount,
			"refund_type":      refundType,
			"refund_method":    refundMethod,
			"refund_reference": refundReference,
			"created_by":       createdByName,
			"created_at":       createdAt,
			"processed_at":     processedAt,
			"notes":            notes,
		})
	}
	return out, total, nil
}

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	order := make(map[string]interface{})
	var branchID, warehouseID, customerID, status, refundType, refundMethod, refundReference, notes, returnNumber, salesInvoiceID, invoiceNumber string
	var branchName, warehouseName, customerName, customerPhone, customerEmail, customerCode, createdByName string
	var soID, soNumber string
	var totalAmount, gstAmount, refundAmount float64
	var createdAt, processedAt interface{}
	if err := s.db.QueryRow(`
		SELECT ro.id, ro.return_number, ro.sales_invoice_id, si.invoice_number,
		       ro.branch_id, ro.warehouse_id, ro.customer_id, ro.status,
		       ro.total_amount, ro.gst_amount, ro.refund_amount, ro.refund_type,
		       ro.refund_method, ro.refund_reference, ro.created_at, ro.processed_at, ro.notes,
		       COALESCE(b.name, '') AS branch_name,
		       COALESCE(w.name, '') AS warehouse_name,
		       COALESCE(c.name, '') AS customer_name,
		       COALESCE(c.phone, '') AS customer_phone,
		       COALESCE(c.email, '') AS customer_email,
		       COALESCE(c.customer_code, '') AS customer_code,
		       COALESCE(u.name, '') AS created_by_name,
		       COALESCE(so.id::text, '') AS so_id,
		       COALESCE(so.so_number, '') AS so_number
		FROM return_orders ro
		JOIN sales_invoices si ON si.id = ro.sales_invoice_id
		LEFT JOIN sales_orders so ON so.id = si.sales_order_id
		LEFT JOIN branches b ON b.id = ro.branch_id
		LEFT JOIN warehouses w ON w.id = ro.warehouse_id
		LEFT JOIN customers c ON c.id = ro.customer_id
		LEFT JOIN users u ON u.id = ro.created_by
		WHERE ro.id = $1
	`, id).Scan(&id, &returnNumber, &salesInvoiceID, &invoiceNumber,
		&branchID, &warehouseID, &customerID, &status,
		&totalAmount, &gstAmount, &refundAmount, &refundType,
		&refundMethod, &refundReference, &createdAt, &processedAt, &notes,
		&branchName, &warehouseName, &customerName, &customerPhone, &customerEmail, &customerCode, &createdByName,
		&soID, &soNumber); err != nil {
		return nil, err
	}
	order["id"] = id
	order["return_number"] = returnNumber
	order["sales_invoice_id"] = salesInvoiceID
	order["invoice_number"] = invoiceNumber
	order["branch_id"] = branchID
	order["branch_name"] = branchName
	order["warehouse_id"] = warehouseID
	order["warehouse_name"] = warehouseName
	order["status"] = status
	order["total_amount"] = totalAmount
	order["gst_amount"] = gstAmount
	order["cgst"] = gstAmount / 2
	order["sgst"] = gstAmount / 2
	order["refund_amount"] = refundAmount
	order["refund_type"] = refundType
	order["refund_method"] = refundMethod
	order["refund_reference"] = refundReference
	order["created_by"] = createdByName
	order["created_at"] = createdAt
	order["processed_at"] = processedAt
	order["notes"] = notes
	order["customer"] = map[string]interface{}{
		"id":            customerID,
		"customer_code": customerCode,
		"name":          customerName,
		"phone":         customerPhone,
		"email":         customerEmail,
	}
	order["sales_order"] = map[string]interface{}{
		"id":        soID,
		"so_number": soNumber,
	}

	// Return items with product/variant details
	itemRows, err := s.db.Query(`
		SELECT ri.id, ri.sales_invoice_item_id, ri.variant_id, ri.quantity, ri.unit_price,
		       ri.discount, ri.bill_discount_share, ri.tax_percent, ri.tax_amount, ri.total_price, ri.reason,
		       v.variant_code, v.sku, v.name AS variant_name, COALESCE(v.barcode, '') AS barcode,
		       p.name AS product_name
		FROM return_items ri
		JOIN variants v ON v.id = ri.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE ri.return_order_id = $1
		ORDER BY ri.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	var items []map[string]interface{}
	for itemRows.Next() {
		var itemID, siItemID, variantID, reason, sku, variantName, barcode, productName string
		var variantCode int
		var quantity float64
		var unitPrice, discount, billDiscountShare, taxPercent, taxAmount, totalPrice float64
		if err := itemRows.Scan(&itemID, &siItemID, &variantID, &quantity,
			&unitPrice, &discount, &billDiscountShare, &taxPercent,
			&taxAmount, &totalPrice, &reason,
			&variantCode, &sku, &variantName, &barcode, &productName); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"id":                    itemID,
			"sales_invoice_item_id": siItemID,
			"variant_id":            variantID,
			"variant_code":          variantCode,
			"variant_name":          variantName,
			"product_name":          productName,
			"sku":                   sku,
			"barcode":               barcode,
			"quantity":              quantity,
			"unit_price":            unitPrice,
			"discount":              discount,
			"bill_discount_share":   billDiscountShare,
			"tax_percent":           taxPercent,
			"cgst_percent":          taxPercent / 2,
			"sgst_percent":          taxPercent / 2,
			"tax_amount":            taxAmount,
			"cgst_amount":           taxAmount / 2,
			"sgst_amount":           taxAmount / 2,
			"total_price":           totalPrice,
			"reason":                reason,
		})
	}
	order["items"] = items

	// Return payments
	payRows, err := s.db.Query(`
		SELECT id, amount, payment_method, COALESCE(reference, ''), paid_at
		FROM return_payments
		WHERE return_order_id = $1
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
	order["payments"] = payments

	return order, nil
}

func (s *Store) nextReturnNumber(tx *sql.Tx) string {
	var maxNo sql.NullString
	err := tx.QueryRow(`SELECT MAX(return_number) FROM return_orders WHERE return_number LIKE 'RTN%'`).Scan(&maxNo)
	if err != nil && err != sql.ErrNoRows {
		return "RTN00001"
	}
	next := 1
	if maxNo.Valid && len(maxNo.String) > 3 {
		fmt.Sscanf(maxNo.String[3:], "%d", &next)
		next++
	}
	return fmt.Sprintf("RTN%05d", next)
}

// CancelReturnOrder sets a return to CANCELLED and reverses stock + invoice adjustments.
func (s *Store) CancelReturnOrder(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var ret struct {
		Status         string
		SalesInvoiceID string
		WarehouseID    string
		TotalAmount    float64
		GSTAmount      float64
		RefundType     string
		RefundAmount   float64
	}
	err = tx.QueryRow(`
		SELECT status, sales_invoice_id, warehouse_id, total_amount, gst_amount, refund_type, refund_amount
		FROM return_orders WHERE id = $1
	`, id).Scan(&ret.Status, &ret.SalesInvoiceID, &ret.WarehouseID,
		&ret.TotalAmount, &ret.GSTAmount, &ret.RefundType, &ret.RefundAmount)
	if err != nil {
		return err
	}
	if ret.Status == ReturnStatusCancelled {
		return fmt.Errorf("return order is already cancelled")
	}

	// Get return items to reverse stock
	rows, err := tx.Query(`
		SELECT ri.variant_id, ri.quantity, ri.unit_price, ri.discount,
		       ri.bill_discount_share, ri.tax_amount, ri.total_price
		FROM return_items ri WHERE ri.return_order_id = $1
	`, id)
	if err != nil {
		return err
	}

	type cancelItem struct {
		variantID                                                string
		quantity, unitPrice, discount, billDisc, taxAmt, totalPx float64
	}
	var cancelItems []cancelItem
	for rows.Next() {
		var ci cancelItem
		if err := rows.Scan(&ci.variantID, &ci.quantity, &ci.unitPrice, &ci.discount,
			&ci.billDisc, &ci.taxAmt, &ci.totalPx); err != nil {
			rows.Close()
			return err
		}
		cancelItems = append(cancelItems, ci)
	}
	rows.Close()

	var totalLineAmount, totalItemDiscount, totalBillDiscount, totalTax, totalRefundAmount float64
	for _, ci := range cancelItems {
		lineTotal := ci.quantity * ci.unitPrice
		totalLineAmount += lineTotal
		totalItemDiscount += ci.discount
		totalBillDiscount += ci.billDisc
		totalTax += ci.taxAmt
		totalRefundAmount += ci.totalPx

		// Reverse stock: subtract the returned quantity back out
		_, err = tx.Exec(`
			UPDATE stocks SET quantity = quantity - $1, updated_at = NOW()
			WHERE variant_id = $2 AND warehouse_id = $3
		`, ci.quantity, ci.variantID, ret.WarehouseID)
		if err != nil {
			return fmt.Errorf("reverse stock: %w", err)
		}
		_, err = tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, from_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1, $2, $3, 'RETURN_CANCEL', $4, 'COMPLETED')
		`, ci.variantID, ret.WarehouseID, ci.quantity, id)
		if err != nil {
			return fmt.Errorf("insert cancel stock movement: %w", err)
		}
	}

	// Restore original invoice amounts
	_, err = tx.Exec(`
		UPDATE sales_invoices
		SET sub_amount = sub_amount + $1,
		    discount_amount = discount_amount + $2,
		    bill_discount = bill_discount + $3,
		    gst_amount = gst_amount + $4,
		    net_amount = net_amount + $5,
		    paid_amount = CASE WHEN $6 = 'CASH' THEN paid_amount + $7 ELSE paid_amount END,
		    updated_at = NOW()
		WHERE id = $8
	`, round2(totalLineAmount), round2(totalItemDiscount), round2(totalBillDiscount),
		round2(totalTax), round2(totalRefundAmount), ret.RefundType, ret.RefundAmount, ret.SalesInvoiceID)
	if err != nil {
		return fmt.Errorf("restore invoice amounts: %w", err)
	}

	// Recalculate invoice status
	var netAmount, paidAmount float64
	if err := tx.QueryRow(`SELECT net_amount, paid_amount FROM sales_invoices WHERE id = $1`,
		ret.SalesInvoiceID).Scan(&netAmount, &paidAmount); err != nil {
		return err
	}
	invoiceStatus := "UNPAID"
	if paidAmount >= netAmount && netAmount > 0 {
		invoiceStatus = "PAID"
	} else if paidAmount > 0 {
		invoiceStatus = "PARTIAL"
	}
	_, err = tx.Exec(`UPDATE sales_invoices SET status = $1 WHERE id = $2`, invoiceStatus, ret.SalesInvoiceID)
	if err != nil {
		return err
	}

	// Check if there are other active returns — if not, restore sales order status
	var activeReturns int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM return_orders
		WHERE sales_invoice_id = $1 AND status != $2 AND id != $3
	`, ret.SalesInvoiceID, ReturnStatusCancelled, id).Scan(&activeReturns)
	if err != nil {
		return err
	}
	if activeReturns == 0 {
		_, err = tx.Exec(`
			UPDATE sales_orders SET status = 'DELIVERED'
			WHERE id = (SELECT sales_order_id FROM sales_invoices WHERE id = $1)
		`, ret.SalesInvoiceID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
		UPDATE return_orders SET status = $1, updated_at = NOW() WHERE id = $2
	`, ReturnStatusCancelled, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}
