package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const variantCacheTTL = 30 * time.Minute

type Store struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewStore(db *sql.DB, rdb *redis.Client) *Store {
	return &Store{db: db, rdb: rdb}
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

	// Calculate totals from items (auto-fetch price if not provided)
	var subtotal, taxTotal, discountTotal, grandTotal float64

	for i, item := range in.Items {
		if item.UnitPrice <= 0 {
			var price float64
			err := tx.QueryRow(`SELECT price FROM variants WHERE id = $1`, item.VariantID).Scan(&price)
			if err != nil {
				return nil, fmt.Errorf("fetch price for variant %s: %w", item.VariantID, err)
			}
			in.Items[i].UnitPrice = price
			item.UnitPrice = price
		}
		lineTotal := float64(item.Quantity) * item.UnitPrice

		// Resolve item discount: percent → flat
		itemDisc := item.Discount
		if item.DiscountType == "percent" {
			itemDisc = lineTotal * item.Discount / 100
		}
		in.Items[i].Discount = itemDisc // store resolved flat amount

		subtotal += lineTotal
		discountTotal += itemDisc
	}

	// Resolve bill discount: percent → flat
	billDiscount := in.BillDiscount
	if in.BillDiscountType == "percent" {
		billDiscount = subtotal * in.BillDiscount / 100
	}
	if billDiscount < 0 {
		billDiscount = 0
	}

	// Bill discount is applied after item discounts, before tax
	taxableAmount := subtotal - discountTotal - billDiscount
	if taxableAmount < 0 {
		taxableAmount = 0
	}

	// Proportionally distribute tax across items based on taxable amount
	for _, item := range in.Items {
		lineTotal := float64(item.Quantity) * item.UnitPrice
		// Proportional share of bill discount for this item
		var itemBillDiscount float64
		if subtotal > 0 {
			itemBillDiscount = billDiscount * lineTotal / subtotal
		}
		lineTaxable := lineTotal - item.Discount - itemBillDiscount
		if lineTaxable < 0 {
			lineTaxable = 0
		}
		lineTax := lineTaxable * item.TaxPercent / 100
		taxTotal += lineTax
	}

	grandTotal = taxableAmount + taxTotal

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
			 subtotal, tax_total, discount_total, bill_discount, grand_total,
			 status, payment_status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'CONFIRMED', $14, $15)
		RETURNING id
	`, soNumber, channel, branchIDParam, customerID, salesPersonParam,
		in.WarehouseID, userID, now,
		subtotal, taxTotal, discountTotal, billDiscount, grandTotal,
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
		var itemBillDisc float64
		if subtotal > 0 {
			itemBillDisc = billDiscount * lineTotal / subtotal
		}
		taxAmt := (lineTotal - item.Discount - itemBillDisc) * item.TaxPercent / 100
		if lineTotal-item.Discount-itemBillDisc < 0 {
			taxAmt = 0
		}
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
			 sub_amount, discount_amount, bill_discount, gst_amount, round_off,
			 net_amount, paid_amount, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 0, $12, $13, $14, $15)
		RETURNING id
	`, salesOrderID, customerID, in.WarehouseID, channel, branchIDParam,
		invoiceNumber, now,
		subtotal, discountTotal, billDiscount, gstAmount,
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

	// ──────────────────────────────────────────
	// Build rich response for receipt/print
	// ──────────────────────────────────────────

	// Fetch branch name
	var branchName string
	if branchID != "" {
		s.db.QueryRow(`SELECT name FROM branches WHERE id = $1`, branchID).Scan(&branchName)
	}

	// Fetch warehouse name
	var warehouseName string
	s.db.QueryRow(`SELECT name FROM warehouses WHERE id = $1`, in.WarehouseID).Scan(&warehouseName)

	// Fetch salesperson name
	var salespersonName string
	if in.SalesPersonID != "" {
		s.db.QueryRow(`SELECT name FROM sales_persons WHERE id = $1`, in.SalesPersonID).Scan(&salespersonName)
	}

	// Build items array with product details
	var responseItems []map[string]interface{}
	for _, ic := range itemCalcs {
		var sku, productName, variantName string
		var variantCode int
		s.db.QueryRow(`
			SELECT COALESCE(v.sku, ''), COALESCE(p.name, ''), COALESCE(v.name, ''), v.variant_code
			FROM variants v
			JOIN products p ON p.id = v.product_id
			WHERE v.id = $1
		`, ic.variantID).Scan(&sku, &productName, &variantName, &variantCode)

		responseItems = append(responseItems, map[string]interface{}{
			"variant_id":   ic.variantID,
			"variant_code": variantCode,
			"sku":          sku,
			"product_name": productName,
			"variant_name": variantName,
			"quantity":     ic.quantity,
			"unit_price":   ic.unitPrice,
			"discount":     ic.discount,
			"tax_percent":  ic.taxPercent,
			"tax_amount":   ic.taxAmount,
			"total_price":  ic.totalPrice,
		})
	}

	// Build payments array
	var responsePayments []map[string]interface{}
	for _, p := range in.Payments {
		responsePayments = append(responsePayments, map[string]interface{}{
			"method":    p.Method,
			"amount":    p.Amount,
			"reference": p.Reference,
		})
	}

	return map[string]interface{}{
		// Identifiers
		"sales_order_id":   salesOrderID,
		"so_number":        soNumber,
		"sales_invoice_id": salesInvoiceID,
		"invoice_number":   invoiceNumber,
		"invoice_date":     now.Format("2006-01-02 15:04:05"),

		// Customer
		"customer_id":    customerID,
		"customer_name":  in.CustomerName,
		"customer_phone": in.CustomerPhone,
		"customer_email": in.CustomerEmail,

		// Context
		"channel":          channel,
		"branch_name":      branchName,
		"warehouse_name":   warehouseName,
		"salesperson_name": salespersonName,

		// Financials
		"subtotal":       subtotal,
		"item_discount":  discountTotal,
		"bill_discount":  billDiscount,
		"total_discount": discountTotal + billDiscount,
		"tax_total":      taxTotal,
		"grand_total":    grandTotal,
		"paid_amount":    totalPaid,
		"balance_due":    grandTotal - totalPaid,
		"payment_status": paymentStatus,

		// Line items & payments
		"items":       responseItems,
		"items_count": len(itemCalcs),
		"payments":    responsePayments,

		// Notes
		"notes": in.Notes,
	}, nil
}

// GetByID returns a bill (sales invoice) with full details.
func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var invoiceID, invoiceNumber, soID, soNumber, customerID, customerName string
	var warehouseID, warehouseName, channel, status, createdAt string
	var branchID, branchName, salespersonName sql.NullString
	var subAmount, discountAmount, billDiscountAmt, gstAmount, roundOff, netAmount, paidAmount float64

	err := s.db.QueryRow(`
		SELECT si.id, si.invoice_number, si.sales_order_id, so.so_number,
		       si.customer_id, c.name AS customer_name,
		       si.warehouse_id, w.name AS warehouse_name,
		       si.channel, si.branch_id, COALESCE(b.name, ''),
		       si.sub_amount, si.discount_amount, si.bill_discount, si.gst_amount,
		       si.round_off, si.net_amount, si.paid_amount,
		       si.status, si.created_at::text,
		       COALESCE(sp.name, '')
		FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		JOIN customers c ON c.id = si.customer_id
		JOIN warehouses w ON w.id = si.warehouse_id
		LEFT JOIN branches b ON b.id = si.branch_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
		WHERE si.id = $1
	`, id).Scan(
		&invoiceID, &invoiceNumber, &soID, &soNumber,
		&customerID, &customerName,
		&warehouseID, &warehouseName,
		&channel, &branchID, &branchName,
		&subAmount, &discountAmount, &billDiscountAmt, &gstAmount,
		&roundOff, &netAmount, &paidAmount,
		&status, &createdAt,
		&salespersonName,
	)
	if err != nil {
		return nil, err
	}

	// Fetch items
	rows, err := s.db.Query(`
		SELECT sii.id, sii.variant_id,
		       COALESCE(v.sku, '') AS sku,
		       COALESCE(p.name, '') AS product_name,
		       v.variant_code,
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
		var variantCode int
		var qty int
		var uPrice, disc, taxPct, taxAmt, totPrice float64
		if err := rows.Scan(&itemID, &variantID, &sku, &productName, &variantCode,
			&qty, &uPrice, &disc, &taxPct, &taxAmt, &totPrice); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"id":           itemID,
			"variant_id":   variantID,
			"variant_code": variantCode,
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
		"id":               invoiceID,
		"invoice_number":   invoiceNumber,
		"sales_order_id":   soID,
		"so_number":        soNumber,
		"customer_id":      customerID,
		"customer_name":    customerName,
		"warehouse_id":     warehouseID,
		"warehouse_name":   warehouseName,
		"channel":          channel,
		"sub_amount":       subAmount,
		"discount_amount":  discountAmount,
		"bill_discount":    billDiscountAmt,
		"gst_amount":       gstAmount,
		"round_off":        roundOff,
		"net_amount":       netAmount,
		"paid_amount":      paidAmount,
		"balance_due":      netAmount - paidAmount,
		"status":           status,
		"created_at":       createdAt,
		"salesperson_name": salespersonName.String,
		"items":            items,
		"payments":         payments,
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
		       si.created_at::text,
		       COALESCE(sp.name, '') AS salesperson_name
		FROM sales_invoices si
		JOIN sales_orders so ON so.id = si.sales_order_id
		JOIN customers c ON c.id = si.customer_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
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
		var id, invNum, soNum, custName, custPhone, channel, status, createdAt, spName string
		var netAmount, paidAmount float64
		if err := rows.Scan(&id, &invNum, &soNum, &custName, &custPhone,
			&channel, &netAmount, &paidAmount, &status, &createdAt, &spName); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":               id,
			"invoice_number":   invNum,
			"so_number":        soNum,
			"customer_name":    custName,
			"customer_phone":   custPhone,
			"channel":          channel,
			"net_amount":       netAmount,
			"paid_amount":      paidAmount,
			"balance_due":      netAmount - paidAmount,
			"salesperson_name": spName,
			"status":           status,
			"created_at":       createdAt,
		})
	}
	return results, nil
}

// GetWarehouseByBranch returns the warehouse ID for a given branch.
func (s *Store) GetWarehouseByBranch(branchID string) (string, error) {
	var warehouseID string
	err := s.db.QueryRow(`
		SELECT id FROM warehouses WHERE branch_id = $1 LIMIT 1
	`, branchID).Scan(&warehouseID)
	return warehouseID, err
}

// GetSalespersonByUserID returns the salesperson ID linked to a user account.
func (s *Store) GetSalespersonByUserID(userID string) (string, error) {
	var spID string
	err := s.db.QueryRow(`
		SELECT id FROM sales_persons WHERE user_id = $1 AND is_active = true
	`, userID).Scan(&spID)
	return spID, err
}

// GetCustomerByPhone returns customer details by phone number.
func (s *Store) GetCustomerByPhone(phone string) (map[string]interface{}, error) {
	var id, code, name, email string
	var ph sql.NullString
	var totalPurchases float64
	var createdAt string

	err := s.db.QueryRow(`
		SELECT id, customer_code, name, COALESCE(phone, ''), COALESCE(email, ''),
		       total_purchases, created_at::text
		FROM customers WHERE phone = $1 AND is_active = true
	`, phone).Scan(&id, &code, &name, &ph, &email, &totalPurchases, &createdAt)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":              id,
		"customer_code":   code,
		"name":            name,
		"phone":           ph.String,
		"email":           email,
		"total_purchases": totalPurchases,
		"created_at":      createdAt,
	}, nil
}

// AddPayment adds a payment to an existing invoice and updates status.
func (s *Store) AddPayment(invoiceID string, p PaymentInput) (map[string]interface{}, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get current invoice totals and linked sales_order_id
	var netAmount, paidAmount float64
	var salesOrderID string
	err = tx.QueryRow(`
		SELECT net_amount, paid_amount, sales_order_id
		FROM sales_invoices WHERE id = $1
	`, invoiceID).Scan(&netAmount, &paidAmount, &salesOrderID)
	if err != nil {
		return nil, err
	}

	balanceDue := netAmount - paidAmount
	if balanceDue <= 0 {
		return nil, fmt.Errorf("invoice is already fully paid")
	}
	if p.Amount > balanceDue {
		return nil, fmt.Errorf("payment amount %.2f exceeds balance due %.2f", p.Amount, balanceDue)
	}

	newPaid := paidAmount + p.Amount

	// Determine new status
	status := "PARTIAL"
	if newPaid >= netAmount {
		status = "PAID"
	}

	// Insert payment record
	now := time.Now()
	_, err = tx.Exec(`
		INSERT INTO sales_payments
			(sales_invoice_id, amount, payment_method, reference, paid_at)
		VALUES ($1, $2, $3, $4, $5)
	`, invoiceID, p.Amount, p.Method, p.Reference, now)
	if err != nil {
		return nil, fmt.Errorf("record payment: %w", err)
	}

	// Update invoice
	_, err = tx.Exec(`
		UPDATE sales_invoices
		SET paid_amount = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`, newPaid, status, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("update invoice: %w", err)
	}

	// Update sales order payment_status
	_, err = tx.Exec(`
		UPDATE sales_orders
		SET payment_status = $1, updated_at = NOW()
		WHERE id = $2
	`, status, salesOrderID)
	if err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"invoice_id":     invoiceID,
		"payment_amount": p.Amount,
		"paid_amount":    newPaid,
		"net_amount":     netAmount,
		"payment_status": status,
	}, nil
}

// QueryLatestSalesPaymentID fetches the most recently created sales_payment ID for an invoice.
func (s *Store) QueryLatestSalesPaymentID(invoiceID string, dest *string) {
	s.db.QueryRow(
		`SELECT id FROM sales_payments WHERE sales_invoice_id = $1 ORDER BY paid_at DESC LIMIT 1`,
		invoiceID,
	).Scan(dest)
}

// LookupVariant searches by SKU or barcode and returns variant details + available stock.
// Variant catalog data is cached in Redis; stock is always fetched live.
func (s *Store) LookupVariant(query, warehouseID string) (map[string]interface{}, error) {
	ctx := context.Background()

	// -- Try Redis cache for variant catalog --
	type variantCache struct {
		VariantID   string  `json:"variant_id"`
		VariantCode int     `json:"variant_code"`
		SKU         string  `json:"sku"`
		Barcode     string  `json:"barcode"`
		VariantName string  `json:"variant_name"`
		ProductName string  `json:"product_name"`
		Price       float64 `json:"price"`
		CostPrice   float64 `json:"cost_price"`
	}

	cacheKey := "variant:lookup:" + query
	var vc variantCache
	cached := false

	if s.rdb != nil {
		val, err := s.rdb.Get(ctx, cacheKey).Result()
		if err == nil {
			if json.Unmarshal([]byte(val), &vc) == nil {
				cached = true
			}
		}
	}

	if !cached {
		// DB lookup
		var barcodeNull sql.NullString
		err := s.db.QueryRow(`
			SELECT v.id, v.variant_code, v.sku, v.name, p.name, v.price, COALESCE(v.cost_price, 0),
			       v.barcode
			FROM variants v
			JOIN products p ON p.id = v.product_id
			WHERE (v.sku = $1 OR v.barcode = $1) AND v.is_active = true
		`, query).Scan(&vc.VariantID, &vc.VariantCode, &vc.SKU, &vc.VariantName, &vc.ProductName,
			&vc.Price, &vc.CostPrice, &barcodeNull)
		if err != nil {
			return nil, err
		}
		if barcodeNull.Valid {
			vc.Barcode = barcodeNull.String
		}

		// Cache in Redis (both by SKU and barcode keys)
		if s.rdb != nil {
			if data, err := json.Marshal(vc); err == nil {
				s.rdb.Set(ctx, "variant:lookup:"+vc.SKU, data, variantCacheTTL)
				if vc.Barcode != "" {
					s.rdb.Set(ctx, "variant:lookup:"+vc.Barcode, data, variantCacheTTL)
				}
			}
		}
	}

	// Stock is ALWAYS fetched live — never cached
	var stock float64
	err := s.db.QueryRow(`
		SELECT COALESCE(quantity, 0)
		FROM stocks
		WHERE variant_id = $1 AND warehouse_id = $2
	`, vc.VariantID, warehouseID).Scan(&stock)
	if err == sql.ErrNoRows {
		stock = 0
	} else if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"variant_id":   vc.VariantID,
		"variant_code": vc.VariantCode,
		"sku":          vc.SKU,
		"barcode":      vc.Barcode,
		"variant_name": vc.VariantName,
		"product_name": vc.ProductName,
		"price":        vc.Price,
		"cost_price":   vc.CostPrice,
		"stock":        stock,
	}, nil
}

// InvalidateVariantCache removes cached variant data when a variant is updated.
func (s *Store) InvalidateVariantCache(sku, barcode string) {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	if sku != "" {
		s.rdb.Del(ctx, "variant:lookup:"+sku)
	}
	if barcode != "" {
		s.rdb.Del(ctx, "variant:lookup:"+barcode)
	}
}

// WarmCache loads all active variants into Redis on startup.
func (s *Store) WarmCache() error {
	if s.rdb == nil {
		return nil
	}

	rows, err := s.db.Query(`
		SELECT v.id, v.variant_code, v.sku, v.name, p.name, v.price, COALESCE(v.cost_price, 0),
		       v.barcode
		FROM variants v
		JOIN products p ON p.id = v.product_id
		WHERE v.is_active = true
	`)
	if err != nil {
		return fmt.Errorf("warm cache query: %w", err)
	}
	defer rows.Close()

	ctx := context.Background()
	count := 0

	type variantCache struct {
		VariantID   string  `json:"variant_id"`
		VariantCode int     `json:"variant_code"`
		SKU         string  `json:"sku"`
		Barcode     string  `json:"barcode"`
		VariantName string  `json:"variant_name"`
		ProductName string  `json:"product_name"`
		Price       float64 `json:"price"`
		CostPrice   float64 `json:"cost_price"`
	}

	for rows.Next() {
		var vc variantCache
		var barcodeNull sql.NullString

		if err := rows.Scan(&vc.VariantID, &vc.VariantCode, &vc.SKU, &vc.VariantName, &vc.ProductName,
			&vc.Price, &vc.CostPrice, &barcodeNull); err != nil {
			return fmt.Errorf("warm cache scan: %w", err)
		}
		if barcodeNull.Valid {
			vc.Barcode = barcodeNull.String
		}

		data, err := json.Marshal(vc)
		if err != nil {
			continue
		}

		// Cache by SKU
		s.rdb.Set(ctx, "variant:lookup:"+vc.SKU, data, variantCacheTTL)
		// Cache by barcode
		if vc.Barcode != "" {
			s.rdb.Set(ctx, "variant:lookup:"+vc.Barcode, data, variantCacheTTL)
		}
		count++
	}

	fmt.Printf("✅ Redis cache warmed: %d variants loaded\n", count)
	return nil
}

// GetCachedVariants returns all cached variant keys and their data from Redis.
func (s *Store) GetCachedVariants() ([]map[string]interface{}, error) {
	if s.rdb == nil {
		return nil, fmt.Errorf("redis not connected")
	}
	ctx := context.Background()

	keys, err := s.rdb.Keys(ctx, "variant:lookup:*").Result()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	seen := map[string]bool{}

	for _, key := range keys {
		val, err := s.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var data map[string]interface{}
		if json.Unmarshal([]byte(val), &data) == nil {
			vid, _ := data["variant_id"].(string)
			if !seen[vid] {
				seen[vid] = true
				results = append(results, data)
			}
		}
	}
	return results, nil
}

// SearchVariants does partial/prefix matching on SKU, barcode, product name, or variant name.
// Tries Redis first, falls back to DB.
func (s *Store) SearchVariants(query string, warehouseID string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	// Always use DB — it's fast with proper query and avoids scanning all Redis keys
	return s.searchFromDB(query, warehouseID, limit)
}

func (s *Store) searchFromRedis(query, warehouseID string, limit int) ([]map[string]interface{}, error) {
	ctx := context.Background()
	keys, err := s.rdb.Keys(ctx, "variant:lookup:*").Result()
	if err != nil {
		return nil, err
	}

	type variantCache struct {
		VariantID   string  `json:"variant_id"`
		VariantCode int     `json:"variant_code"`
		SKU         string  `json:"sku"`
		Barcode     string  `json:"barcode"`
		VariantName string  `json:"variant_name"`
		ProductName string  `json:"product_name"`
		Price       float64 `json:"price"`
		CostPrice   float64 `json:"cost_price"`
	}

	seen := map[string]bool{}
	var results []map[string]interface{}
	queryLower := strings.ToLower(query)

	for _, key := range keys {
		if len(results) >= limit {
			break
		}
		val, err := s.rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var vc variantCache
		if json.Unmarshal([]byte(val), &vc) != nil {
			continue
		}
		if seen[vc.VariantID] {
			continue
		}

		// Match against SKU, barcode, product name, variant name, variant code
		variantCodeStr := strconv.Itoa(vc.VariantCode)
		if strings.Contains(strings.ToLower(vc.SKU), queryLower) ||
			strings.Contains(strings.ToLower(vc.Barcode), queryLower) ||
			strings.Contains(strings.ToLower(vc.ProductName), queryLower) ||
			strings.Contains(strings.ToLower(vc.VariantName), queryLower) ||
			strings.Contains(variantCodeStr, query) {

			seen[vc.VariantID] = true

			// Fetch live stock
			var stock float64
			err := s.db.QueryRow(`
				SELECT COALESCE(quantity, 0) FROM stocks
				WHERE variant_id = $1 AND warehouse_id = $2
			`, vc.VariantID, warehouseID).Scan(&stock)
			if err != nil {
				stock = 0
			}

			results = append(results, map[string]interface{}{
				"variant_id":   vc.VariantID,
				"variant_code": vc.VariantCode,
				"sku":          vc.SKU,
				"barcode":      vc.Barcode,
				"variant_name": vc.VariantName,
				"product_name": vc.ProductName,
				"price":        vc.Price,
				"cost_price":   vc.CostPrice,
				"stock":        stock,
			})
		}
	}
	return results, nil
}

func (s *Store) searchFromDB(query, warehouseID string, limit int) ([]map[string]interface{}, error) {
	// Check if query is a pure number (variant code search)
	isNumeric := true
	for _, c := range query {
		if c < '0' || c > '9' {
			isNumeric = false
			break
		}
	}

	var rows *sql.Rows
	var err error

	if isNumeric {
		// Exact match on variant_code for numeric queries
		code, _ := strconv.Atoi(query)
		rows, err = s.db.Query(`
			SELECT v.id, v.variant_code, v.sku, COALESCE(v.barcode, ''), v.name, p.name,
			       v.price, COALESCE(v.cost_price, 0),
			       COALESCE(st.quantity, 0)
			FROM variants v
			JOIN products p ON p.id = v.product_id
			LEFT JOIN stocks st ON st.variant_id = v.id AND st.warehouse_id = $1
			WHERE v.is_active = true AND v.variant_code = $2
			LIMIT $3
		`, warehouseID, code, limit)
	} else {
		// Text search on SKU, barcode, product name, variant name
		pattern := "%" + query + "%"
		rows, err = s.db.Query(`
			SELECT v.id, v.variant_code, v.sku, COALESCE(v.barcode, ''), v.name, p.name,
			       v.price, COALESCE(v.cost_price, 0),
			       COALESCE(st.quantity, 0)
			FROM variants v
			JOIN products p ON p.id = v.product_id
			LEFT JOIN stocks st ON st.variant_id = v.id AND st.warehouse_id = $1
			WHERE v.is_active = true
			  AND (v.sku ILIKE $2 OR v.barcode ILIKE $2 OR p.name ILIKE $2 OR v.name ILIKE $2)
			LIMIT $3
		`, warehouseID, pattern, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var variantID, sku, barcode, variantName, productName string
		var variantCode int
		var price, costPrice, stock float64
		if err := rows.Scan(&variantID, &variantCode, &sku, &barcode, &variantName, &productName,
			&price, &costPrice, &stock); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"variant_id":   variantID,
			"variant_code": variantCode,
			"sku":          sku,
			"barcode":      barcode,
			"variant_name": variantName,
			"product_name": productName,
			"price":        price,
			"cost_price":   costPrice,
			"stock":        stock,
		})
	}
	return results, nil
}
