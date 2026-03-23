package billing

import (
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateBill handles the entire billing flow in a single transaction:
// 1. Find or create customer
// 2. Create sales order + items
// 3. Create sales invoice + items
// 4. Record payments
// 5. Deduct stock + create stock movements
// 6. Update customer total_purchases
func (s *Store) CreateBill(in CreateBillInput, userID, branchID string) (map[string]interface{}, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()

	// ──────────────────────────────────────────
	// 1. Find or create customer
	// ──────────────────────────────────────────
	var customerID string
	err = tx.QueryRow(
		`SELECT id FROM customers WHERE phone = $1`, in.CustomerPhone,
	).Scan(&customerID)

	if err == sql.ErrNoRows {
		// Auto-generate customer code
		var maxCode sql.NullString
		tx.QueryRow(`SELECT MAX(customer_code) FROM customers WHERE customer_code LIKE 'CUS%'`).Scan(&maxCode)
		next := 1
		if maxCode.Valid && len(maxCode.String) > 3 {
			fmt.Sscanf(maxCode.String[3:], "%d", &next)
			next++
		}
		code := fmt.Sprintf("CUS%04d", next)

		err = tx.QueryRow(`
			INSERT INTO customers (customer_code, name, phone, email)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`, code, in.CustomerName, in.CustomerPhone, in.CustomerEmail).Scan(&customerID)
		if err != nil {
			return nil, fmt.Errorf("create customer: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("find customer: %w", err)
	}

	// ──────────────────────────────────────────
	// 2. Create sales order
	// ──────────────────────────────────────────
	var maxSO sql.NullString
	tx.QueryRow(`SELECT MAX(so_number) FROM sales_orders WHERE so_number LIKE 'SO%'`).Scan(&maxSO)
	soNext := 1
	if maxSO.Valid && len(maxSO.String) > 2 {
		fmt.Sscanf(maxSO.String[2:], "%d", &soNext)
		soNext++
	}
	soNumber := fmt.Sprintf("SO%05d", soNext)

	channel := in.Channel
	if channel == "" {
		channel = "STORE"
	}

	// Calculate totals from items
	var subtotal, taxTotal, discountTotal, grandTotal float64
	for _, item := range in.Items {
		lineTotal := float64(item.Quantity) * item.UnitPrice
		lineTax := (lineTotal - item.Discount) * item.TaxPercent / 100
		subtotal += lineTotal
		discountTotal += item.Discount
		taxTotal += lineTax
	}
	grandTotal = subtotal - discountTotal + taxTotal

	// Determine payment status
	var totalPaid float64
	for _, p := range in.Payments {
		totalPaid += p.Amount
	}
	paymentStatus := "UNPAID"
	if totalPaid >= grandTotal {
		paymentStatus = "PAID"
	} else if totalPaid > 0 {
		paymentStatus = "PARTIAL"
	}

	var branchIDParam interface{}
	if branchID != "" {
		branchIDParam = branchID
	}

	var salesPersonParam interface{}
	if in.SalesPersonID != "" {
		salesPersonParam = in.SalesPersonID
	}

	var salesOrderID string
	err = tx.QueryRow(`
		INSERT INTO sales_orders
			(so_number, channel, branch_id, customer_id, salesperson_id,
			 warehouse_id, created_by, order_date,
			 subtotal, tax_total, discount_total, grand_total,
			 status, payment_status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'CONFIRMED', $13, $14)
		RETURNING id
	`, soNumber, channel, branchIDParam, customerID, salesPersonParam,
		in.WarehouseID, userID, now,
		subtotal, taxTotal, discountTotal, grandTotal,
		paymentStatus, in.Notes).Scan(&salesOrderID)
	if err != nil {
		return nil, fmt.Errorf("create sales order: %w", err)
	}

	// ──────────────────────────────────────────
	// 3. Create sales order items
	// ──────────────────────────────────────────
	type itemCalc struct {
		variantID  string
		quantity   int
		unitPrice  float64
		discount   float64
		taxPercent float64
		taxAmount  float64
		totalPrice float64
	}
	var itemCalcs []itemCalc

	for _, item := range in.Items {
		lineTotal := float64(item.Quantity) * item.UnitPrice
		taxAmt := (lineTotal - item.Discount) * item.TaxPercent / 100
		total := lineTotal - item.Discount + taxAmt

		ic := itemCalc{
			variantID:  item.VariantID,
			quantity:   item.Quantity,
			unitPrice:  item.UnitPrice,
			discount:   item.Discount,
			taxPercent: item.TaxPercent,
			taxAmount:  taxAmt,
			totalPrice: total,
		}
		itemCalcs = append(itemCalcs, ic)

		_, err = tx.Exec(`
			INSERT INTO sales_order_items
				(sales_order_id, variant_id, quantity, unit_price, discount, tax_percent, tax_amount, total_price)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, salesOrderID, item.VariantID, item.Quantity, item.UnitPrice,
			item.Discount, item.TaxPercent, taxAmt, total)
		if err != nil {
			return nil, fmt.Errorf("create sales order item: %w", err)
		}
	}

	// ──────────────────────────────────────────
	// 4. Create sales invoice
	// ──────────────────────────────────────────
	var maxInv sql.NullString
	tx.QueryRow(`SELECT MAX(invoice_number) FROM sales_invoices WHERE invoice_number LIKE 'INV%'`).Scan(&maxInv)
	invNext := 1
	if maxInv.Valid && len(maxInv.String) > 3 {
		fmt.Sscanf(maxInv.String[3:], "%d", &invNext)
		invNext++
	}
	invoiceNumber := fmt.Sprintf("INV%05d", invNext)

	gstAmount := taxTotal
	netAmount := grandTotal

	invoiceStatus := paymentStatus
	if invoiceStatus == "PAID" {
		invoiceStatus = "PAID"
	} else if invoiceStatus == "PARTIAL" {
		invoiceStatus = "PARTIAL"
	} else {
		invoiceStatus = "UNPAID"
	}

	var salesInvoiceID string
	err = tx.QueryRow(`
		INSERT INTO sales_invoices
			(sales_order_id, customer_id, warehouse_id, channel, branch_id,
			 invoice_number, invoice_date,
			 sub_amount, discount_amount, gst_amount, round_off,
			 net_amount, paid_amount, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 0, $11, $12, $13, $14)
		RETURNING id
	`, salesOrderID, customerID, in.WarehouseID, channel, branchIDParam,
		invoiceNumber, now,
		subtotal, discountTotal, gstAmount,
		netAmount, totalPaid, invoiceStatus, userID).Scan(&salesInvoiceID)
	if err != nil {
		return nil, fmt.Errorf("create sales invoice: %w", err)
	}

	// ──────────────────────────────────────────
	// 5. Create sales invoice items
	// ──────────────────────────────────────────
	for _, ic := range itemCalcs {
		_, err = tx.Exec(`
			INSERT INTO sales_invoice_items
				(sales_invoice_id, variant_id, quantity, unit_price, discount, tax_percent, tax_amount, total_price)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, salesInvoiceID, ic.variantID, ic.quantity, ic.unitPrice,
			ic.discount, ic.taxPercent, ic.taxAmount, ic.totalPrice)
		if err != nil {
			return nil, fmt.Errorf("create sales invoice item: %w", err)
		}
	}

	// ──────────────────────────────────────────
	// 6. Record payments
	// ──────────────────────────────────────────
	for _, p := range in.Payments {
		_, err = tx.Exec(`
			INSERT INTO sales_payments
				(sales_invoice_id, amount, payment_method, reference, paid_at)
			VALUES ($1, $2, $3, $4, $5)
		`, salesInvoiceID, p.Amount, p.Method, p.Reference, now)
		if err != nil {
			return nil, fmt.Errorf("record payment: %w", err)
		}
	}

	// ──────────────────────────────────────────
	// 7. Deduct stock + create movements
	// ──────────────────────────────────────────
	for _, ic := range itemCalcs {
		// Deduct from stock
		res, err := tx.Exec(`
			UPDATE stocks
			SET quantity = quantity - $1, updated_at = NOW()
			WHERE variant_id = $2 AND warehouse_id = $3 AND quantity >= $1
		`, ic.quantity, ic.variantID, in.WarehouseID)
		if err != nil {
			return nil, fmt.Errorf("deduct stock: %w", err)
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			// Get variant name for error message
			var variantName string
			tx.QueryRow(`
				SELECT COALESCE(v.sku, p.name)
				FROM variants v
				JOIN products p ON p.id = v.product_id
				WHERE v.id = $1
			`, ic.variantID).Scan(&variantName)
			return nil, fmt.Errorf("insufficient stock for %s", variantName)
		}

		// Create stock movement (OUT)
		_, err = tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, from_warehouse_id, quantity, movement_type,
				 sale_order_id, status, reference, created_at)
			VALUES ($1, $2, $3, 'OUT', $4, 'COMPLETED', $5, $6)
		`, ic.variantID, in.WarehouseID, ic.quantity,
			salesOrderID, "SALE:"+invoiceNumber, now)
		if err != nil {
			return nil, fmt.Errorf("create stock movement: %w", err)
		}
	}

	// ──────────────────────────────────────────
	// 8. Update customer total_purchases
	// ──────────────────────────────────────────
	_, err = tx.Exec(`
		UPDATE customers
		SET total_purchases = total_purchases + $1, updated_at = NOW()
		WHERE id = $2
	`, grandTotal, customerID)
	if err != nil {
		return nil, fmt.Errorf("update customer total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Return summary
	return map[string]interface{}{
		"sales_order_id":   salesOrderID,
		"so_number":        soNumber,
		"sales_invoice_id": salesInvoiceID,
		"invoice_number":   invoiceNumber,
		"customer_id":      customerID,
		"subtotal":         subtotal,
		"discount_total":   discountTotal,
		"tax_total":        taxTotal,
		"grand_total":      grandTotal,
		"paid_amount":      totalPaid,
		"payment_status":   paymentStatus,
		"items_count":      len(itemCalcs),
	}, nil
}

// GetByID returns a bill (sales invoice) with full details.
func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var invoiceID, invoiceNumber, soID, soNumber, customerID, customerName string
	var warehouseID, warehouseName, channel, status, createdAt string
	var branchID, branchName sql.NullString
	var subAmount, discountAmount, gstAmount, roundOff, netAmount, paidAmount float64

	err := s.db.QueryRow(`
		SELECT si.id, si.invoice_number, si.sales_order_id, so.so_number,
		       si.customer_id, c.name AS customer_name,
		       si.warehouse_id, w.name AS warehouse_name,
		       si.channel, si.branch_id, COALESCE(b.name, ''),
		       si.sub_amount, si.discount_amount, si.gst_amount,
		       si.round_off, si.net_amount, si.paid_amount,
		       si.status, si.created_at::text
		FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		JOIN customers c ON c.id = si.customer_id
		JOIN warehouses w ON w.id = si.warehouse_id
		LEFT JOIN branches b ON b.id = si.branch_id
		WHERE si.id = $1
	`, id).Scan(
		&invoiceID, &invoiceNumber, &soID, &soNumber,
		&customerID, &customerName,
		&warehouseID, &warehouseName,
		&channel, &branchID, &branchName,
		&subAmount, &discountAmount, &gstAmount,
		&roundOff, &netAmount, &paidAmount,
		&status, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	// Fetch items
	rows, err := s.db.Query(`
		SELECT sii.id, sii.variant_id,
		       COALESCE(v.sku, '') AS sku,
		       COALESCE(p.name, '') AS product_name,
		       sii.quantity, sii.unit_price, sii.discount,
		       sii.tax_percent, sii.tax_amount, sii.total_price
		FROM sales_invoice_items sii
		JOIN variants v ON v.id = sii.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE sii.sales_invoice_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var itemID, variantID, sku, productName string
		var qty int
		var uPrice, disc, taxPct, taxAmt, totPrice float64
		if err := rows.Scan(&itemID, &variantID, &sku, &productName,
			&qty, &uPrice, &disc, &taxPct, &taxAmt, &totPrice); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"id":           itemID,
			"variant_id":   variantID,
			"sku":          sku,
			"product_name": productName,
			"quantity":     qty,
			"unit_price":   uPrice,
			"discount":     disc,
			"tax_percent":  taxPct,
			"tax_amount":   taxAmt,
			"total_price":  totPrice,
		})
	}

	// Fetch payments
	payRows, err := s.db.Query(`
		SELECT id, amount, payment_method, COALESCE(reference, ''), paid_at::text
		FROM sales_payments
		WHERE sales_invoice_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer payRows.Close()

	var payments []map[string]interface{}
	for payRows.Next() {
		var payID, method, ref, paidAt string
		var amount float64
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

	result := map[string]interface{}{
		"id":              invoiceID,
		"invoice_number":  invoiceNumber,
		"sales_order_id":  soID,
		"so_number":       soNumber,
		"customer_id":     customerID,
		"customer_name":   customerName,
		"warehouse_id":    warehouseID,
		"warehouse_name":  warehouseName,
		"channel":         channel,
		"sub_amount":      subAmount,
		"discount_amount": discountAmount,
		"gst_amount":      gstAmount,
		"round_off":       roundOff,
		"net_amount":      netAmount,
		"paid_amount":     paidAmount,
		"status":          status,
		"created_at":      createdAt,
		"items":           items,
		"payments":        payments,
	}

	if branchID.Valid {
		result["branch_id"] = branchID.String
		result["branch_name"] = branchName.String
	}

	return result, nil
}

// List returns all bills with pagination. Filters by branch for StoreManager.
func (s *Store) List(branchID *string, limit, offset int) ([]map[string]interface{}, error) {
	query := `
		SELECT si.id, si.invoice_number, so.so_number,
		       c.name AS customer_name, c.phone AS customer_phone,
		       si.channel, si.net_amount, si.paid_amount, si.status,
		       si.created_at::text
		FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		JOIN customers c ON c.id = si.customer_id
	`
	args := []interface{}{}
	argIdx := 1

	if branchID != nil {
		query += fmt.Sprintf(" WHERE si.branch_id = $%d", argIdx)
		args = append(args, *branchID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY si.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, invNum, soNum, custName, custPhone, channel, status, createdAt string
		var netAmount, paidAmount float64
		if err := rows.Scan(&id, &invNum, &soNum, &custName, &custPhone,
			&channel, &netAmount, &paidAmount, &status, &createdAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":             id,
			"invoice_number": invNum,
			"so_number":      soNum,
			"customer_name":  custName,
			"customer_phone": custPhone,
			"channel":        channel,
			"net_amount":     netAmount,
			"paid_amount":    paidAmount,
			"status":         status,
			"created_at":     createdAt,
		})
	}
	return results, nil
}
