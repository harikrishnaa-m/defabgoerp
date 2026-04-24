package order

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Checkout creates an order from the customer's cart.
func (s *Store) Checkout(customerID string, in CheckoutInput) (map[string]interface{}, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Get cart items with variant details
	rows, err := tx.Query(`
		SELECT ci.variant_id, v.variant_code, v.name, v.sku, v.price,
		       p.name AS product_name, ci.quantity
		FROM ecom_cart_items ci
		JOIN ecom_carts c ON c.id = ci.cart_id
		JOIN variants v ON v.id = ci.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE c.customer_id = $1
		ORDER BY ci.created_at
	`, customerID)
	if err != nil {
		return nil, err
	}

	type cartItem struct {
		VariantID   string
		VariantCode int
		VariantName string
		SKU         string
		Price       float64
		ProductName string
		Quantity    int
	}

	var items []cartItem
	for rows.Next() {
		var ci cartItem
		if err := rows.Scan(&ci.VariantID, &ci.VariantCode, &ci.VariantName, &ci.SKU,
			&ci.Price, &ci.ProductName, &ci.Quantity); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, ci)
	}
	rows.Close()

	if len(items) == 0 {
		return nil, fmt.Errorf("cart is empty")
	}

	// 2. Calculate totals
	var subTotal float64
	var itemCount int
	for _, it := range items {
		subTotal += it.Price * float64(it.Quantity)
		itemCount += it.Quantity
	}

	shippingCharge := in.ShippingCharge
	grandTotal := subTotal - 0 + 0 + shippingCharge // discount=0, tax=0 for now
	grandTotal = math.Round(grandTotal*100) / 100

	// 3. Snapshot the shipping address
	var shippingName, shippingPhone, shippingAddr, shippingCity, shippingState, shippingPincode string
	if in.AddressID != "" {
		err = tx.QueryRow(`
			SELECT full_name, phone, address_line1 || COALESCE(', ' || NULLIF(address_line2,''), ''),
			       city, state, pincode
			FROM ecom_addresses
			WHERE id = $1 AND customer_id = $2
		`, in.AddressID, customerID).Scan(&shippingName, &shippingPhone, &shippingAddr,
			&shippingCity, &shippingState, &shippingPincode)
		if err != nil {
			return nil, fmt.Errorf("address not found")
		}
	}

	// 4. Generate order number
	var seq int
	tx.QueryRow(`SELECT nextval('ecom_order_seq')`).Scan(&seq)
	orderNumber := fmt.Sprintf("ECOM-%05d", seq)

	// 5. Insert order
	var orderID string
	err = tx.QueryRow(`
		INSERT INTO ecom_orders (
			order_number, customer_id, address_id,
			shipping_name, shipping_phone, shipping_address,
			shipping_city, shipping_state, shipping_pincode,
			item_count, sub_total, discount_amount, tax_amount,
			shipping_charge, grand_total,
			payment_method, notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id
	`, orderNumber, customerID, nilIfEmpty(in.AddressID),
		shippingName, shippingPhone, shippingAddr,
		shippingCity, shippingState, shippingPincode,
		itemCount, subTotal, 0, 0,
		shippingCharge, grandTotal,
		in.PaymentMethod, in.Notes,
	).Scan(&orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// 6. Insert order items
	for _, it := range items {
		lineTotal := it.Price * float64(it.Quantity)
		_, err = tx.Exec(`
			INSERT INTO ecom_order_items (order_id, variant_id, product_name, variant_name, sku, variant_code, quantity, unit_price, total_price)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`, orderID, it.VariantID, it.ProductName, it.VariantName, it.SKU, it.VariantCode, it.Quantity, it.Price, lineTotal)
		if err != nil {
			return nil, fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	// 7. Check and decrement online_stocks (row-level lock to prevent oversell)
	for _, it := range items {
		var onlineQty int
		err = tx.QueryRow(`SELECT quantity FROM online_stocks WHERE variant_id = $1 FOR UPDATE`, it.VariantID).Scan(&onlineQty)
		if err != nil {
			return nil, fmt.Errorf("online stock not available for '%s'", it.VariantName)
		}
		if onlineQty < it.Quantity {
			return nil, fmt.Errorf("insufficient online stock for '%s' (available: %d, requested: %d)", it.VariantName, onlineQty, it.Quantity)
		}
		_, err = tx.Exec(`UPDATE online_stocks SET quantity = quantity - $1, updated_at = NOW() WHERE variant_id = $2`, it.Quantity, it.VariantID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrement online stock: %w", err)
		}
	}

	// 8. Clear the cart
	tx.Exec(`
		DELETE FROM ecom_cart_items
		WHERE cart_id = (SELECT id FROM ecom_carts WHERE customer_id = $1)
	`, customerID)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"order_id":     orderID,
		"order_number": orderNumber,
		"item_count":   itemCount,
		"sub_total":    subTotal,
		"grand_total":  grandTotal,
		"status":       "PENDING",
	}, nil
}

// ListOrders returns paginated orders for a customer.
func (s *Store) ListOrders(customerID string, page, limit int) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * limit

	var total int
	s.db.QueryRow(`SELECT COUNT(*) FROM ecom_orders WHERE customer_id = $1`, customerID).Scan(&total)

	rows, err := s.db.Query(`
		SELECT id, order_number, item_count, grand_total,
		       status, payment_status, payment_method, created_at
		FROM ecom_orders
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, customerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []map[string]interface{}
	for rows.Next() {
		var id, orderNum, status, payStatus string
		var payMethod sql.NullString
		var itemCount int
		var grandTotal float64
		var createdAt time.Time

		rows.Scan(&id, &orderNum, &itemCount, &grandTotal,
			&status, &payStatus, &payMethod, &createdAt)

		pm := ""
		if payMethod.Valid {
			pm = payMethod.String
		}

		orders = append(orders, map[string]interface{}{
			"id":             id,
			"order_number":   orderNum,
			"item_count":     itemCount,
			"grand_total":    grandTotal,
			"status":         status,
			"payment_status": payStatus,
			"payment_method": pm,
			"created_at":     createdAt,
		})
	}
	return orders, total, nil
}

// GetOrder returns full order details.
func (s *Store) GetOrder(customerID, orderID string) (map[string]interface{}, error) {
	var id, orderNum, status, payStatus string
	var payMethod, payRef, notes sql.NullString
	var shippingName, shippingPhone, shippingAddr, shippingCity, shippingState, shippingPincode sql.NullString
	var itemCount int
	var subTotal, discountAmt, taxAmt, shippingCharge, grandTotal float64
	var createdAt, updatedAt time.Time

	err := s.db.QueryRow(`
		SELECT id, order_number, item_count,
		       sub_total, discount_amount, tax_amount, shipping_charge, grand_total,
		       status, payment_status, payment_method, payment_ref, notes,
		       shipping_name, shipping_phone, shipping_address,
		       shipping_city, shipping_state, shipping_pincode,
		       created_at, updated_at
		FROM ecom_orders
		WHERE id = $1 AND customer_id = $2
	`, orderID, customerID).Scan(
		&id, &orderNum, &itemCount,
		&subTotal, &discountAmt, &taxAmt, &shippingCharge, &grandTotal,
		&status, &payStatus, &payMethod, &payRef, &notes,
		&shippingName, &shippingPhone, &shippingAddr,
		&shippingCity, &shippingState, &shippingPincode,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	order := map[string]interface{}{
		"id":              id,
		"order_number":    orderNum,
		"item_count":      itemCount,
		"sub_total":       subTotal,
		"discount_amount": discountAmt,
		"tax_amount":      taxAmt,
		"shipping_charge": shippingCharge,
		"grand_total":     grandTotal,
		"status":          status,
		"payment_status":  payStatus,
		"payment_method":  nullStr(payMethod),
		"payment_ref":     nullStr(payRef),
		"notes":           nullStr(notes),
		"shipping_address": map[string]interface{}{
			"name":    nullStr(shippingName),
			"phone":   nullStr(shippingPhone),
			"address": nullStr(shippingAddr),
			"city":    nullStr(shippingCity),
			"state":   nullStr(shippingState),
			"pincode": nullStr(shippingPincode),
		},
		"created_at": createdAt,
		"updated_at": updatedAt,
	}

	// Fetch items
	itemRows, err := s.db.Query(`
		SELECT id, variant_id, product_name, variant_name, sku, variant_code,
		       quantity, unit_price, total_price
		FROM ecom_order_items
		WHERE order_id = $1
		ORDER BY created_at
	`, orderID)
	if err != nil {
		return order, nil
	}
	defer itemRows.Close()

	var items []map[string]interface{}
	for itemRows.Next() {
		var iid, vid, pName, vName, sku string
		var vCode, qty int
		var uPrice, tPrice float64
		itemRows.Scan(&iid, &vid, &pName, &vName, &sku, &vCode, &qty, &uPrice, &tPrice)

		items = append(items, map[string]interface{}{
			"id":           iid,
			"variant_id":   vid,
			"product_name": pName,
			"variant_name": vName,
			"sku":          sku,
			"variant_code": vCode,
			"quantity":     qty,
			"unit_price":   uPrice,
			"total_price":  tPrice,
		})
	}
	order["items"] = items

	return order, nil
}

// CancelOrder allows customer to cancel a PENDING order.
func (s *Store) CancelOrder(customerID, orderID string) error {
	res, err := s.db.Exec(`
		UPDATE ecom_orders SET status = 'CANCELLED', updated_at = NOW()
		WHERE id = $1 AND customer_id = $2 AND status = 'PENDING'
	`, orderID, customerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order not found or cannot be cancelled")
	}
	return nil
}

// ── Admin helpers (for ERP staff to manage ecom orders) ─────

// AdminListOrders lists all ecom orders with filters.
func (s *Store) AdminListOrders(status string, page, limit int) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * limit

	where := ""
	args := []interface{}{}
	idx := 1

	if status != "" {
		where = fmt.Sprintf(" WHERE o.status = $%d", idx)
		args = append(args, status)
		idx++
	}

	var total int
	countQ := "SELECT COUNT(*) FROM ecom_orders o" + where
	s.db.QueryRow(countQ, args...).Scan(&total)

	query := fmt.Sprintf(`
		SELECT o.id, o.order_number, ec.name, ec.email, ec.phone,
		       o.item_count, o.grand_total, o.status, o.payment_status,
		       o.created_at
		FROM ecom_orders o
		JOIN ecom_customers ec ON ec.id = o.customer_id
		%s
		ORDER BY o.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []map[string]interface{}
	for rows.Next() {
		var id, orderNum, custName, custEmail, sts, paySts string
		var custPhone sql.NullString
		var itemCount int
		var grandTotal float64
		var createdAt time.Time

		rows.Scan(&id, &orderNum, &custName, &custEmail, &custPhone,
			&itemCount, &grandTotal, &sts, &paySts, &createdAt)

		orders = append(orders, map[string]interface{}{
			"id":             id,
			"order_number":   orderNum,
			"customer_name":  custName,
			"customer_email": custEmail,
			"customer_phone": nullStr(custPhone),
			"item_count":     itemCount,
			"grand_total":    grandTotal,
			"status":         sts,
			"payment_status": paySts,
			"created_at":     createdAt,
		})
	}
	return orders, total, nil
}

// AdminUpdateStatus updates the order status (CONFIRMED, PROCESSING, SHIPPED, DELIVERED, CANCELLED).
func (s *Store) AdminUpdateStatus(orderID, status string) error {
	res, err := s.db.Exec(`
		UPDATE ecom_orders SET status = $2, updated_at = NOW() WHERE id = $1
	`, orderID, status)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order not found")
	}
	return nil
}

// AdminUpdatePayment updates payment status and reference.
func (s *Store) AdminUpdatePayment(orderID, payStatus, payRef string) error {
	res, err := s.db.Exec(`
		UPDATE ecom_orders SET payment_status = $2, payment_ref = $3, updated_at = NOW()
		WHERE id = $1
	`, orderID, payStatus, payRef)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("order not found")
	}
	return nil
}

// AdminGetOrder returns full order details without customer_id check (for admins).
func (s *Store) AdminGetOrder(orderID string) (map[string]interface{}, error) {
	var id, orderNum, custID, status, payStatus string
	var payMethod, payRef, notes sql.NullString
	var shippingName, shippingPhone, shippingAddr, shippingCity, shippingState, shippingPincode sql.NullString
	var itemCount int
	var subTotal, discountAmt, taxAmt, shippingCharge, grandTotal float64
	var createdAt, updatedAt time.Time

	err := s.db.QueryRow(`
		SELECT o.id, o.order_number, o.customer_id, o.item_count,
		       o.sub_total, o.discount_amount, o.tax_amount, o.shipping_charge, o.grand_total,
		       o.status, o.payment_status, o.payment_method, o.payment_ref, o.notes,
		       o.shipping_name, o.shipping_phone, o.shipping_address,
		       o.shipping_city, o.shipping_state, o.shipping_pincode,
		       o.created_at, o.updated_at
		FROM ecom_orders o
		WHERE o.id = $1
	`, orderID).Scan(
		&id, &orderNum, &custID, &itemCount,
		&subTotal, &discountAmt, &taxAmt, &shippingCharge, &grandTotal,
		&status, &payStatus, &payMethod, &payRef, &notes,
		&shippingName, &shippingPhone, &shippingAddr,
		&shippingCity, &shippingState, &shippingPincode,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Fetch customer info
	var custName, custEmail string
	var custPhone sql.NullString
	s.db.QueryRow(`SELECT name, email, phone FROM ecom_customers WHERE id = $1`, custID).Scan(&custName, &custEmail, &custPhone)

	order := map[string]interface{}{
		"id":           id,
		"order_number": orderNum,
		"customer": map[string]interface{}{
			"id":    custID,
			"name":  custName,
			"email": custEmail,
			"phone": nullStr(custPhone),
		},
		"item_count":      itemCount,
		"sub_total":       subTotal,
		"discount_amount": discountAmt,
		"tax_amount":      taxAmt,
		"shipping_charge": shippingCharge,
		"grand_total":     grandTotal,
		"status":          status,
		"payment_status":  payStatus,
		"payment_method":  nullStr(payMethod),
		"payment_ref":     nullStr(payRef),
		"notes":           nullStr(notes),
		"shipping_address": map[string]interface{}{
			"name":    nullStr(shippingName),
			"phone":   nullStr(shippingPhone),
			"address": nullStr(shippingAddr),
			"city":    nullStr(shippingCity),
			"state":   nullStr(shippingState),
			"pincode": nullStr(shippingPincode),
		},
		"created_at": createdAt,
		"updated_at": updatedAt,
	}

	// Fetch items
	itemRows, err := s.db.Query(`
		SELECT id, variant_id, product_name, variant_name, sku, variant_code,
		       quantity, unit_price, total_price
		FROM ecom_order_items WHERE order_id = $1 ORDER BY created_at
	`, orderID)
	if err != nil {
		return order, nil
	}
	defer itemRows.Close()

	var items []map[string]interface{}
	for itemRows.Next() {
		var iid, vid, pName, vName, sku string
		var vCode, qty int
		var uPrice, tPrice float64
		itemRows.Scan(&iid, &vid, &pName, &vName, &sku, &vCode, &qty, &uPrice, &tPrice)

		items = append(items, map[string]interface{}{
			"id":           iid,
			"variant_id":   vid,
			"product_name": pName,
			"variant_name": vName,
			"sku":          sku,
			"variant_code": vCode,
			"quantity":     qty,
			"unit_price":   uPrice,
			"total_price":  tPrice,
		})
	}
	order["items"] = items

	return order, nil
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
