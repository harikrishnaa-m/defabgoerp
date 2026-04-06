package returns

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

func (s *Store) CreateReturnOrder(in CreateReturnOrderInput, userID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var invoice struct {
		ID           string
		SalesOrderID string
		CustomerID   string
		BranchID     sql.NullString
		WarehouseID  string
		SubAmount    float64
		BillDiscount float64
		GSTAmount    float64
		NetAmount    float64
		PaidAmount   float64
	}
	if err := tx.QueryRow(`
		SELECT id, sales_order_id, customer_id, branch_id, warehouse_id,
		       sub_amount, bill_discount, gst_amount, net_amount, paid_amount
		FROM sales_invoices WHERE id = $1
	`, in.SalesInvoiceID).Scan(&invoice.ID, &invoice.SalesOrderID, &invoice.CustomerID,
		&invoice.BranchID, &invoice.WarehouseID, &invoice.SubAmount, &invoice.BillDiscount,
		&invoice.GSTAmount, &invoice.NetAmount, &invoice.PaidAmount); err != nil {
		return "", fmt.Errorf("sales invoice lookup: %w", err)
	}

	type stockUpdate struct {
		variantID string
		quantity  int
	}
	var stockUpdates []stockUpdate
	totalAmount := 0.0
	gstAmount := 0.0
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
			Quantity   int
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
			return "", fmt.Errorf("return quantity %d exceeds invoiced quantity %d", item.Quantity, siItem.Quantity)
		}

		var alreadyReturned int
		if err := tx.QueryRow(`
			SELECT COALESCE(SUM(quantity), 0)
			FROM return_items ri
			JOIN return_orders ro ON ri.return_order_id = ro.id
			WHERE ro.sales_invoice_id = $1 AND ri.sales_invoice_item_id = $2 AND ro.status != $3
		`, invoice.ID, siItem.ID, ReturnStatusCancelled).Scan(&alreadyReturned); err != nil {
			return "", fmt.Errorf("return quantity check: %w", err)
		}
		if alreadyReturned+item.Quantity > siItem.Quantity {
			return "", fmt.Errorf("cannot return more than invoiced quantity for item %s", siItem.ID)
		}

		lineTotal := float64(item.Quantity) * siItem.UnitPrice
		lineBillDisc := 0.0
		if invoice.SubAmount > 0 {
			lineBillDisc = (lineTotal * invoice.BillDiscount) / invoice.SubAmount
		}
		taxable := lineTotal - siItem.Discount - lineBillDisc
		if taxable < 0 {
			taxable = 0
		}
		lineTax := taxable * siItem.TaxPercent / 100
		lineReturnAmount := lineTotal - siItem.Discount - lineBillDisc + lineTax

		_, err := tx.Exec(`
			INSERT INTO return_items
				(return_order_id, sales_invoice_item_id, variant_id, quantity, unit_price,
				 discount, bill_discount_share, tax_percent, tax_amount, total_price, reason)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, returnID, siItem.ID, siItem.VariantID, item.Quantity, siItem.UnitPrice,
			siItem.Discount, lineBillDisc, siItem.TaxPercent, lineTax, lineReturnAmount, item.Reason)
		if err != nil {
			return "", fmt.Errorf("insert return item: %w", err)
		}

		stockUpdates = append(stockUpdates, stockUpdate{variantID: siItem.VariantID, quantity: item.Quantity})
		totalAmount += lineReturnAmount
		gstAmount += lineTax
	}

	if err := tx.QueryRow(`
		UPDATE return_orders
		SET total_amount = $1, gst_amount = $2
		WHERE id = $3
		RETURNING id
	`, totalAmount, gstAmount, returnID).Scan(&returnID); err != nil {
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
		refundAmount = totalAmount
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

	invoice.SubAmount -= totalAmount - gstAmount
	invoice.GSTAmount -= gstAmount
	invoice.NetAmount -= totalAmount
	if invoice.SubAmount < 0 {
		invoice.SubAmount = 0
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
	`, invoice.SubAmount, 0.0, invoice.BillDiscount,
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

func (s *Store) CompleteReturnOrder(returnID string, in CompleteReturnInput, userID string) (map[string]interface{}, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var order struct {
		ID             string
		ReturnNumber   string
		SalesInvoiceID string
		BranchID       sql.NullString
		WarehouseID    string
		CustomerID     string
		TotalAmount    float64
		GSTAmount      float64
		Status         string
	}
	if err := tx.QueryRow(`
		SELECT id, return_number, sales_invoice_id, branch_id, warehouse_id, customer_id,
		       total_amount, gst_amount, status
		FROM return_orders WHERE id = $1
	`, returnID).Scan(&order.ID, &order.ReturnNumber, &order.SalesInvoiceID,
		&order.BranchID, &order.WarehouseID, &order.CustomerID,
		&order.TotalAmount, &order.GSTAmount, &order.Status); err != nil {
		return nil, err
	}
	if order.Status != ReturnStatusRequested {
		return nil, fmt.Errorf("return order is not in requested state")
	}

	rows, err := tx.Query(`
		SELECT ri.variant_id, ri.quantity, ri.total_price
		FROM return_items ri
		WHERE ri.return_order_id = $1
	`, returnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var variantID string
		var qty int
		var totalPrice float64
		if err := rows.Scan(&variantID, &qty, &totalPrice); err != nil {
			return nil, err
		}
		_, err = tx.Exec(`
			INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
			VALUES ($1,$2,$3,'PRODUCT', NOW())
			ON CONFLICT (variant_id, warehouse_id)
			DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity, updated_at = NOW()
		`, variantID, order.WarehouseID, qty)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(`
			INSERT INTO stock_movements
				(variant_id, to_warehouse_id, quantity, movement_type, reference, status)
			VALUES ($1, $2, $3, 'RETURN_IN', $4, 'COMPLETED')
		`, variantID, order.WarehouseID, qty, order.ID)
		if err != nil {
			return nil, err
		}
	}

	refundType := in.RefundType
	if refundType == "" {
		refundType = RefundTypeCash
	}
	if refundType != RefundTypeCash && refundType != RefundTypeCredit {
		return nil, fmt.Errorf("invalid refund_type %s", refundType)
	}

	refundAmount := 0.0
	if refundType == RefundTypeCash {
		refundAmount = order.TotalAmount
		if in.RefundMethod == "" {
			return nil, fmt.Errorf("refund_method is required for cash refunds")
		}
	}

	if refundAmount > 0 {
		_, err = tx.Exec(`
			INSERT INTO return_payments
				(return_order_id, amount, payment_method, reference, paid_at)
			VALUES ($1,$2,$3,$4,NOW())
		`, order.ID, refundAmount, in.RefundMethod, in.RefundReference)
		if err != nil {
			return nil, err
		}
	}

	var invoice struct {
		ID           string
		SalesOrderID string
		SubAmount    float64
		DiscountAmt  float64
		BillDiscount float64
		GSTAmount    float64
		NetAmount    float64
		PaidAmount   float64
	}
	if err := tx.QueryRow(`
		SELECT id, sales_order_id, sub_amount, discount_amount, bill_discount, gst_amount, net_amount, paid_amount
		FROM sales_invoices WHERE id = $1
	`, order.SalesInvoiceID).Scan(&invoice.ID, &invoice.SalesOrderID,
		&invoice.SubAmount, &invoice.DiscountAmt, &invoice.BillDiscount,
		&invoice.GSTAmount, &invoice.NetAmount, &invoice.PaidAmount); err != nil {
		return nil, err
	}

	invoice.SubAmount -= order.TotalAmount - order.GSTAmount
	invoice.DiscountAmt -= 0
	invoice.BillDiscount -= 0
	invoice.GSTAmount -= order.GSTAmount
	invoice.NetAmount -= order.TotalAmount
	if invoice.SubAmount < 0 {
		invoice.SubAmount = 0
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
	`, invoice.SubAmount, invoice.DiscountAmt, invoice.BillDiscount,
		invoice.GSTAmount, invoice.NetAmount, invoice.PaidAmount,
		invoiceStatus, invoice.ID)
	if err != nil {
		return nil, err
	}

	orderStatus := "PARTIAL_RETURN"
	if invoice.NetAmount == 0 {
		orderStatus = "RETURNED"
	}
	_, err = tx.Exec(`
		UPDATE sales_orders SET status = $1 WHERE id = $2
	`, orderStatus, invoice.SalesOrderID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		UPDATE return_orders
		SET status = $1, refund_type = $2, refund_method = $3,
		    refund_reference = $4, refund_amount = $5, processed_at = NOW()
		WHERE id = $6
	`, ReturnStatusCompleted, refundType, in.RefundMethod, in.RefundReference, refundAmount, order.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"return_order_id": order.ID,
		"status":          ReturnStatusCompleted,
		"refund_amount":   refundAmount,
	}, nil
}

func (s *Store) CancelReturnOrder(returnID string) error {
	res, err := s.db.Exec(`UPDATE return_orders SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3`,
		ReturnStatusCancelled, returnID, ReturnStatusRequested)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
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
		       ro.refund_method, ro.refund_reference, ro.created_at, ro.processed_at, ro.notes
		FROM return_orders ro
		JOIN sales_invoices si ON si.id = ro.sales_invoice_id
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
		var totalAmount, gstAmount, refundAmount float64
		var createdAt, processedAt interface{}
		if err := rows.Scan(&roID, &roNumber, &salesInvoiceID, &invoiceNumber,
			&branchIDVal, &warehouseID, &customerID, &status,
			&totalAmount, &gstAmount, &refundAmount, &refundType,
			&refundMethod, &refundReference, &createdAt, &processedAt, &notes); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]interface{}{
			"id":               roID,
			"return_number":    roNumber,
			"sales_invoice_id": salesInvoiceID,
			"invoice_number":   invoiceNumber,
			"branch_id":        branchIDVal,
			"warehouse_id":     warehouseID,
			"customer_id":      customerID,
			"status":           status,
			"total_amount":     totalAmount,
			"gst_amount":       gstAmount,
			"refund_amount":    refundAmount,
			"refund_type":      refundType,
			"refund_method":    refundMethod,
			"refund_reference": refundReference,
			"created_at":       createdAt,
			"processed_at":     processedAt,
			"notes":            notes,
		})
	}
	return out, total, nil
}

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var order map[string]interface{}
	order = make(map[string]interface{})
	var branchID, warehouseID, customerID, status, refundType, refundMethod, refundReference, notes, returnNumber, salesInvoiceID, invoiceNumber string
	var totalAmount, gstAmount, refundAmount float64
	var createdAt, processedAt interface{}
	if err := s.db.QueryRow(`
		SELECT ro.id, ro.return_number, ro.sales_invoice_id, si.invoice_number,
		       ro.branch_id, ro.warehouse_id, ro.customer_id, ro.status,
		       ro.total_amount, ro.gst_amount, ro.refund_amount, ro.refund_type,
		       ro.refund_method, ro.refund_reference, ro.created_at, ro.processed_at, ro.notes
		FROM return_orders ro
		JOIN sales_invoices si ON si.id = ro.sales_invoice_id
		WHERE ro.id = $1
	`, id).Scan(&id, &returnNumber, &salesInvoiceID, &invoiceNumber,
		&branchID, &warehouseID, &customerID, &status,
		&totalAmount, &gstAmount, &refundAmount, &refundType,
		&refundMethod, &refundReference, &createdAt, &processedAt, &notes); err != nil {
		return nil, err
	}
	order["id"] = id
	order["return_number"] = returnNumber
	order["sales_invoice_id"] = salesInvoiceID
	order["invoice_number"] = invoiceNumber
	order["branch_id"] = branchID
	order["warehouse_id"] = warehouseID
	order["customer_id"] = customerID
	order["status"] = status
	order["total_amount"] = totalAmount
	order["gst_amount"] = gstAmount
	order["refund_amount"] = refundAmount
	order["refund_type"] = refundType
	order["refund_method"] = refundMethod
	order["refund_reference"] = refundReference
	order["created_at"] = createdAt
	order["processed_at"] = processedAt
	order["notes"] = notes

	itemRows, err := s.db.Query(`
		SELECT id, sales_invoice_item_id, variant_id, quantity, unit_price,
		       discount, bill_discount_share, tax_percent, tax_amount, total_price, reason
		FROM return_items
		WHERE return_order_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	var items []map[string]interface{}
	for itemRows.Next() {
		var itemID, siItemID, variantID, reason string
		var quantity int
		var unitPrice, discount, billDiscountShare, taxPercent, taxAmount, totalPrice float64
		if err := itemRows.Scan(&itemID, &siItemID, &variantID, &quantity,
			&unitPrice, &discount, &billDiscountShare, &taxPercent,
			&taxAmount, &totalPrice, &reason); err != nil {
			return nil, err
		}
		items = append(items, map[string]interface{}{
			"id":                    itemID,
			"sales_invoice_item_id": siItemID,
			"variant_id":            variantID,
			"quantity":              quantity,
			"unit_price":            unitPrice,
			"discount":              discount,
			"bill_discount_share":   billDiscountShare,
			"tax_percent":           taxPercent,
			"tax_amount":            taxAmount,
			"total_price":           totalPrice,
			"reason":                reason,
		})
	}
	order["items"] = items

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
