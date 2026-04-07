package salesorder

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

func (s *Store) List(branchID *string, status, paymentStatus string, limit, offset int) ([]map[string]interface{}, int, error) {
	baseWhere := "WHERE 1=1"
	var args []interface{}
	argIdx := 1

	if branchID != nil {
		baseWhere += fmt.Sprintf(" AND so.branch_id = $%d", argIdx)
		args = append(args, *branchID)
		argIdx++
	}

	if status != "" {
		baseWhere += fmt.Sprintf(" AND so.status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	if paymentStatus != "" {
		baseWhere += fmt.Sprintf(" AND so.payment_status = $%d", argIdx)
		args = append(args, paymentStatus)
		argIdx++
	}

	// Count
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM sales_orders so %s`, baseWhere)
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List
	query := fmt.Sprintf(`
		SELECT
			so.id, so.so_number, so.channel, so.order_date,
			so.subtotal, so.tax_total, so.discount_total, so.bill_discount, so.grand_total,
			so.status, so.payment_status, so.notes,
			so.created_at,
			COALESCE(c.name, '') AS customer_name,
			COALESCE(c.phone, '') AS customer_phone,
			COALESCE(b.name, '') AS branch_name,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(w.name, '') AS warehouse_name,
			COALESCE(si.invoice_number, '') AS invoice_number,
			COALESCE(si.net_amount, 0) AS net_amount,
			COALESCE(si.paid_amount, 0) AS paid_amount,
			COALESCE(si.status, '') AS invoice_status
		FROM sales_orders so
		LEFT JOIN customers c ON c.id = so.customer_id
		LEFT JOIN branches b ON b.id = so.branch_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
		LEFT JOIN warehouses w ON w.id = so.warehouse_id
		LEFT JOIN sales_invoices si ON si.sales_order_id = so.id
		%s
		ORDER BY so.created_at DESC
		LIMIT $%d OFFSET $%d
	`, baseWhere, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		var id, soNumber, channel, status, paymentStatus, notes string
		var customerName, customerPhone, branchName, spName, whName string
		var invoiceNumber, invoiceStatus string
		var orderDate, createdAt interface{}
		var subtotal, taxTotal, discountTotal, billDiscount, grandTotal float64
		var netAmount, paidAmount float64

		if err := rows.Scan(
			&id, &soNumber, &channel, &orderDate,
			&subtotal, &taxTotal, &discountTotal, &billDiscount, &grandTotal,
			&status, &paymentStatus, &notes,
			&createdAt,
			&customerName, &customerPhone,
			&branchName, &spName, &whName,
			&invoiceNumber, &netAmount, &paidAmount, &invoiceStatus,
		); err != nil {
			return nil, 0, err
		}

		row := map[string]interface{}{
			"id":               id,
			"so_number":        soNumber,
			"channel":          channel,
			"order_date":       orderDate,
			"subtotal":         subtotal,
			"tax_total":        taxTotal,
			"cgst":             taxTotal / 2,
			"sgst":             taxTotal / 2,
			"discount_total":   discountTotal,
			"bill_discount":    billDiscount,
			"grand_total":      grandTotal,
			"status":           status,
			"payment_status":   paymentStatus,
			"notes":            notes,
			"customer_name":    customerName,
			"customer_phone":   customerPhone,
			"branch_name":      branchName,
			"salesperson_name": spName,
			"warehouse_name":   whName,
			"created_at":       createdAt,
		}

		if invoiceNumber != "" {
			row["invoice"] = map[string]interface{}{
				"invoice_number": invoiceNumber,
				"net_amount":     netAmount,
				"paid_amount":    paidAmount,
				"balance_due":    netAmount - paidAmount,
				"status":         invoiceStatus,
			}
		}

		out = append(out, row)
	}

	return out, total, nil
}

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	// Main order with all joins
	var soID, soNumber, channel, status, paymentStatus, notes string
	var customerID, customerName, customerPhone, customerEmail, customerCode string
	var branchName, spName, whName, createdByName string
	var orderDate, createdAt, updatedAt interface{}
	var subtotal, taxTotal, discountTotal, billDiscount, grandTotal float64

	err := s.db.QueryRow(`
		SELECT
			so.id, so.so_number, so.channel, so.order_date,
			so.subtotal, so.tax_total, so.discount_total, so.bill_discount, so.grand_total,
			so.status, so.payment_status, COALESCE(so.notes, ''),
			so.created_at, so.updated_at,
			c.id, COALESCE(c.customer_code, ''), c.name, COALESCE(c.phone, ''), COALESCE(c.email, ''),
			COALESCE(b.name, '') AS branch_name,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(w.name, '') AS warehouse_name,
			COALESCE(u.name, '') AS created_by_name
		FROM sales_orders so
		JOIN customers c ON c.id = so.customer_id
		LEFT JOIN branches b ON b.id = so.branch_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
		LEFT JOIN warehouses w ON w.id = so.warehouse_id
		LEFT JOIN users u ON u.id = so.created_by
		WHERE so.id = $1
	`, id).Scan(
		&soID, &soNumber, &channel, &orderDate,
		&subtotal, &taxTotal, &discountTotal, &billDiscount, &grandTotal,
		&status, &paymentStatus, &notes,
		&createdAt, &updatedAt,
		&customerID, &customerCode, &customerName, &customerPhone, &customerEmail,
		&branchName, &spName, &whName, &createdByName,
	)
	if err != nil {
		return nil, err
	}

	order := map[string]interface{}{
		"id":               soID,
		"so_number":        soNumber,
		"channel":          channel,
		"order_date":       orderDate,
		"subtotal":         subtotal,
		"tax_total":        taxTotal,
		"cgst":             taxTotal / 2,
		"sgst":             taxTotal / 2,
		"discount_total":   discountTotal,
		"bill_discount":    billDiscount,
		"grand_total":      grandTotal,
		"status":           status,
		"payment_status":   paymentStatus,
		"notes":            notes,
		"branch_name":      branchName,
		"salesperson_name": spName,
		"warehouse_name":   whName,
		"created_by":       createdByName,
		"created_at":       createdAt,
		"updated_at":       updatedAt,
		"customer": map[string]interface{}{
			"id":            customerID,
			"customer_code": customerCode,
			"name":          customerName,
			"phone":         customerPhone,
			"email":         customerEmail,
		},
	}

	// Items with product/variant details
	itemRows, err := s.db.Query(`
		SELECT
			soi.id, soi.quantity, soi.unit_price, soi.discount,
			soi.tax_percent, soi.tax_amount, soi.total_price,
			v.variant_code, v.sku, v.name AS variant_name, COALESCE(v.barcode, '') AS barcode,
			p.name AS product_name
		FROM sales_order_items soi
		JOIN variants v ON v.id = soi.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE soi.sales_order_id = $1
		ORDER BY soi.id
	`, id)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	var items []map[string]interface{}
	for itemRows.Next() {
		var itemID, sku, variantName, barcode, productName string
		var variantCode int
		var quantity float64
		var unitPrice, discount, taxPercent, taxAmount, totalPrice float64

		if err := itemRows.Scan(
			&itemID, &quantity, &unitPrice, &discount,
			&taxPercent, &taxAmount, &totalPrice,
			&variantCode, &sku, &variantName, &barcode, &productName,
		); err != nil {
			return nil, err
		}

		items = append(items, map[string]interface{}{
			"id":           itemID,
			"product_name": productName,
			"variant_name": variantName,
			"variant_code": variantCode,
			"sku":          sku,
			"barcode":      barcode,
			"quantity":     quantity,
			"unit_price":   unitPrice,
			"discount":     discount,
			"tax_percent":  taxPercent,
			"cgst_percent": taxPercent / 2,
			"sgst_percent": taxPercent / 2,
			"tax_amount":   taxAmount,
			"cgst_amount":  taxAmount / 2,
			"sgst_amount":  taxAmount / 2,
			"total_price":  totalPrice,
		})
	}
	order["items"] = items

	// Invoice details
	var invID, invNumber, invStatus string
	var invDate interface{}
	var subAmount, discountAmount, gstAmount, roundOff, netAmount, paidAmount float64

	err = s.db.QueryRow(`
		SELECT id, invoice_number, invoice_date,
		       sub_amount, discount_amount, gst_amount, round_off,
		       net_amount, paid_amount, bill_discount, status
		FROM sales_invoices
		WHERE sales_order_id = $1
	`, id).Scan(
		&invID, &invNumber, &invDate,
		&subAmount, &discountAmount, &gstAmount, &roundOff,
		&netAmount, &paidAmount, &billDiscount, &invStatus,
	)
	if err == nil {
		invoice := map[string]interface{}{
			"id":              invID,
			"invoice_number":  invNumber,
			"invoice_date":    invDate,
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
			"status":          invStatus,
		}

		// Payments
		payRows, err := s.db.Query(`
			SELECT id, amount, payment_method, COALESCE(reference, ''), paid_at
			FROM sales_payments
			WHERE sales_invoice_id = $1
			ORDER BY paid_at
		`, invID)
		if err == nil {
			defer payRows.Close()
			var payments []map[string]interface{}
			for payRows.Next() {
				var payID, method, ref string
				var amount float64
				var paidAt interface{}
				if err := payRows.Scan(&payID, &amount, &method, &ref, &paidAt); err == nil {
					payments = append(payments, map[string]interface{}{
						"id":             payID,
						"amount":         amount,
						"payment_method": method,
						"reference":      ref,
						"paid_at":        paidAt,
					})
				}
			}
			invoice["payments"] = payments
		}

		order["invoice"] = invoice
	}

	return order, nil
}
