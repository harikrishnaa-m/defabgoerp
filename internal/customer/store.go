package customer

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

func (s *Store) List(limit, offset int, search string) ([]map[string]interface{}, int, error) {
	baseWhere := "WHERE 1=1"
	var args []interface{}
	argIdx := 1

	if search != "" {
		baseWhere += fmt.Sprintf(" AND (c.name ILIKE $%d OR c.phone ILIKE $%d OR c.email ILIKE $%d OR c.customer_code ILIKE $%d)", argIdx, argIdx, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	// Count
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM customers c %s`, baseWhere)
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// List
	query := fmt.Sprintf(`
		SELECT
			c.id, c.customer_code, c.name, c.phone, c.email,
			c.total_purchases, c.is_active, c.created_at, c.updated_at,
			COUNT(DISTINCT so.id) AS order_count
		FROM customers c
		LEFT JOIN sales_orders so ON so.customer_id = c.id
		%s
		GROUP BY c.id
		ORDER BY c.created_at DESC
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
		var id, code, name, phone, email string
		var totalPurchases float64
		var isActive bool
		var createdAt, updatedAt interface{}
		var orderCount int

		if err := rows.Scan(&id, &code, &name, &phone, &email, &totalPurchases, &isActive, &createdAt, &updatedAt, &orderCount); err != nil {
			return nil, 0, err
		}

		out = append(out, map[string]interface{}{
			"id":              id,
			"customer_code":   code,
			"name":            name,
			"phone":           phone,
			"email":           email,
			"total_purchases": totalPurchases,
			"is_active":       isActive,
			"created_at":      createdAt,
			"updated_at":      updatedAt,
			"order_count":     orderCount,
		})
	}

	return out, total, nil
}

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	// Customer details
	var custID, code, name, phone, email string
	var totalPurchases float64
	var isActive bool
	var createdAt, updatedAt interface{}

	err := s.db.QueryRow(`
		SELECT id, customer_code, name, phone, email,
		       total_purchases, is_active, created_at, updated_at
		FROM customers WHERE id = $1
	`, id).Scan(&custID, &code, &name, &phone, &email, &totalPurchases, &isActive, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	customer := map[string]interface{}{
		"id":              custID,
		"customer_code":   code,
		"name":            name,
		"phone":           phone,
		"email":           email,
		"total_purchases": totalPurchases,
		"is_active":       isActive,
		"created_at":      createdAt,
		"updated_at":      updatedAt,
	}

	// Recent orders with invoice + payment info
	orderRows, err := s.db.Query(`
		SELECT
			so.id, so.so_number, so.channel,
			so.order_date, so.subtotal, so.tax_total,
			so.discount_total, so.grand_total,
			so.status, so.payment_status,
			COALESCE(b.name, '') AS branch_name,
			COALESCE(sp.name, '') AS salesperson_name,
			COALESCE(si.invoice_number, '') AS invoice_number,
			COALESCE(si.net_amount, 0) AS net_amount,
			COALESCE(si.paid_amount, 0) AS paid_amount,
			COALESCE(si.status, '') AS invoice_status,
			so.created_at
		FROM sales_orders so
		LEFT JOIN branches b ON b.id = so.branch_id
		LEFT JOIN sales_persons sp ON sp.id = so.salesperson_id
		LEFT JOIN sales_invoices si ON si.sales_order_id = so.id
		WHERE so.customer_id = $1
		ORDER BY so.created_at DESC
		LIMIT 20
	`, id)
	if err != nil {
		return nil, err
	}
	defer orderRows.Close()

	var orders []map[string]interface{}
	for orderRows.Next() {
		var soID, soNumber, channel, status, paymentStatus, branchName, spName string
		var invoiceNumber, invoiceStatus string
		var orderDate, soCreatedAt interface{}
		var subtotal, taxTotal, discountTotal, grandTotal, netAmount, paidAmount float64

		if err := orderRows.Scan(
			&soID, &soNumber, &channel,
			&orderDate, &subtotal, &taxTotal,
			&discountTotal, &grandTotal,
			&status, &paymentStatus,
			&branchName, &spName,
			&invoiceNumber, &netAmount, &paidAmount, &invoiceStatus,
			&soCreatedAt,
		); err != nil {
			return nil, err
		}

		order := map[string]interface{}{
			"id":               soID,
			"so_number":        soNumber,
			"channel":          channel,
			"order_date":       orderDate,
			"subtotal":         subtotal,
			"tax_total":        taxTotal,
			"discount_total":   discountTotal,
			"grand_total":      grandTotal,
			"status":           status,
			"payment_status":   paymentStatus,
			"branch_name":      branchName,
			"salesperson_name": spName,
			"created_at":       soCreatedAt,
		}

		if invoiceNumber != "" {
			order["invoice"] = map[string]interface{}{
				"invoice_number": invoiceNumber,
				"net_amount":     netAmount,
				"paid_amount":    paidAmount,
				"balance_due":    netAmount - paidAmount,
				"status":         invoiceStatus,
			}
		}

		orders = append(orders, order)
	}

	customer["orders"] = orders

	return customer, nil
}
