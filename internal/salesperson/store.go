package salesperson

import (
	"database/sql"
	"fmt"
	"log"

	"defab-erp/internal/auth"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(in CreateSalesPersonInput) (string, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	// Hash password
	hashed, err := auth.HashPassword(in.Password)
	if err != nil {
		return "", "", fmt.Errorf("hash password: %w", err)
	}

	// Get SalesPerson role_id
	var roleID int
	err = tx.QueryRow(`SELECT id FROM roles WHERE name = 'SalesPerson'`).Scan(&roleID)
	if err != nil {
		return "", "", fmt.Errorf("SalesPerson role not found: %w", err)
	}

	// Create user account
	var userID string
	err = tx.QueryRow(`
		INSERT INTO users (name, email, password_hash, role_id, branch_id, is_active)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		RETURNING id
	`, in.Name, in.Email, hashed, roleID, in.BranchID).Scan(&userID)
	if err != nil {
		return "", "", fmt.Errorf("create user: %w", err)
	}

	// Auto-generate employee code
	var maxCode sql.NullString
	tx.QueryRow(`SELECT MAX(employee_code) FROM sales_persons WHERE employee_code LIKE 'SP%'`).Scan(&maxCode)

	next := 1
	if maxCode.Valid && len(maxCode.String) > 2 {
		fmt.Sscanf(maxCode.String[2:], "%d", &next)
		next++
	}
	code := fmt.Sprintf("SP%03d", next)

	// Create salesperson linked to user
	var id string
	err = tx.QueryRow(`
		INSERT INTO sales_persons
			(employee_code, user_id, branch_id, name, phone, email)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, code, userID, in.BranchID, in.Name, in.Phone, in.Email).Scan(&id)
	if err != nil {
		return "", "", fmt.Errorf("create salesperson: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return id, code, nil
}

func (s *Store) List(branchID *string, limit, offset int) ([]map[string]interface{}, error) {
	query := `
		SELECT sp.id, sp.employee_code, sp.name, sp.phone, sp.email,
		       sp.is_active, sp.user_id, sp.branch_id,
		       COALESCE(b.name, '') AS branch_name,
		       sp.created_at::text
		FROM sales_persons sp
		LEFT JOIN branches b ON b.id = sp.branch_id
	`
	args := []interface{}{}
	argIdx := 1

	if branchID != nil {
		query += fmt.Sprintf(" WHERE sp.branch_id = $%d", argIdx)
		args = append(args, *branchID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY sp.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, code, name, createdAt string
		var phone, email sql.NullString
		var active bool
		var userID, brID sql.NullString
		var branchName string

		if err := rows.Scan(&id, &code, &name, &phone, &email, &active, &userID, &brID, &branchName, &createdAt); err != nil {
			return nil, err
		}

		item := map[string]interface{}{
			"id":            id,
			"employee_code": code,
			"name":          name,
			"phone":         nullStr(phone),
			"email":         nullStr(email),
			"is_active":     active,
			"user_id":       nullStr(userID),
			"branch_id":     nullStr(brID),
			"branch_name":   branchName,
			"created_at":    createdAt,
		}
		results = append(results, item)
	}
	return results, nil
}

func (s *Store) GetByID(id string, f SalesFilter) (map[string]interface{}, error) {
	var spID, code, name, createdAt string
	var phone, email, userID, brID sql.NullString
	var branchName string
	var active bool

	err := s.db.QueryRow(`
		SELECT sp.id, sp.employee_code, sp.name, sp.phone, sp.email,
		       sp.is_active, sp.user_id, sp.branch_id,
		       COALESCE(b.name, '') AS branch_name,
		       sp.created_at::text
		FROM sales_persons sp
		LEFT JOIN branches b ON b.id = sp.branch_id
		WHERE sp.id = $1
	`, id).Scan(&spID, &code, &name, &phone, &email, &active, &userID, &brID, &branchName, &createdAt)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":            spID,
		"employee_code": code,
		"name":          name,
		"phone":         nullStr(phone),
		"email":         nullStr(email),
		"is_active":     active,
		"user_id":       nullStr(userID),
		"branch_id":     nullStr(brID),
		"branch_name":   branchName,
		"created_at":    createdAt,
	}

	// ── Attendance details (if salesperson has a linked user) ──
	if userID.Valid {
		attendance, err := s.getAttendanceDetails(userID.String)
		if err == nil {
			result["attendance"] = attendance
		}
	}

	// ── Sales report ──
	salesReport, err := s.getSalesReport(id, f)
	if err != nil {
		log.Println("getSalesReport error:", err)
	} else {
		result["sales_report"] = salesReport
	}

	return result, nil
}

func (s *Store) getAttendanceDetails(uid string) (map[string]interface{}, error) {
	// Last 30 attendance records
	rows, err := s.db.Query(`
		SELECT a.date::text, a.punch_in::text, COALESCE(a.punch_out::text,'') AS punch_out,
		       a.total_hours, a.notes
		FROM attendance a
		WHERE a.user_id = $1
		ORDER BY a.date DESC
		LIMIT 30
	`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []map[string]interface{}
	for rows.Next() {
		var date, punchIn, punchOut, notes string
		var totalHours float64
		if err := rows.Scan(&date, &punchIn, &punchOut, &totalHours, &notes); err != nil {
			continue
		}
		rec := map[string]interface{}{
			"date":        date,
			"punch_in":    punchIn,
			"total_hours": totalHours,
			"notes":       notes,
		}
		if punchOut != "" {
			rec["punch_out"] = punchOut
		} else {
			rec["punch_out"] = nil
		}
		records = append(records, rec)
	}
	if records == nil {
		records = []map[string]interface{}{}
	}

	// Summary
	var totalHours float64
	var daysPresent int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(total_hours),0), COUNT(*)
		FROM attendance WHERE user_id = $1
	`, uid).Scan(&totalHours, &daysPresent)

	return map[string]interface{}{
		"recent_records": records,
		"total_hours":    totalHours,
		"days_present":   daysPresent,
	}, nil
}

func (s *Store) getSalesReport(salespersonID string, f SalesFilter) (map[string]interface{}, error) {
	// ── Build dynamic WHERE for date & category filters ──
	baseJoin := `FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id`
	where := ` WHERE so.salesperson_id = $1`
	args := []interface{}{salespersonID}
	n := 1

	if f.CategoryID != "" {
		baseJoin += `
		JOIN sales_order_items soi ON soi.sales_order_id = so.id
		JOIN variants v ON v.id = soi.variant_id
		JOIN products p ON p.id = v.product_id`
		n++
		where += fmt.Sprintf(` AND p.category_id = $%d`, n)
		args = append(args, f.CategoryID)
	}
	if f.From != "" {
		n++
		where += fmt.Sprintf(` AND so.created_at >= $%d::date`, n)
		args = append(args, f.From)
	}
	if f.To != "" {
		n++
		where += fmt.Sprintf(` AND so.created_at < ($%d::date + INTERVAL '1 day')`, n)
		args = append(args, f.To)
	}

	// Use DISTINCT when category join may duplicate rows
	countExpr := "COUNT(*)"
	if f.CategoryID != "" {
		countExpr = "COUNT(DISTINCT so.id)"
	}

	// ── Summary (filtered) ──
	var totalOrders int
	var totalAmount float64
	filteredQ := fmt.Sprintf(`SELECT %s, COALESCE(SUM(si.net_amount),0) %s %s`, countExpr, baseJoin, where)
	err := s.db.QueryRow(filteredQ, args...).Scan(&totalOrders, &totalAmount)
	if err != nil {
		return nil, err
	}

	// ── Overall summary (always unfiltered) ──
	var allOrders int
	var allAmount float64
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(si.net_amount),0)
		FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id
		WHERE so.salesperson_id = $1
	`, salespersonID).Scan(&allOrders, &allAmount)

	// ── This month (unfiltered) ──
	var monthOrders int
	var monthAmount float64
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(si.net_amount),0)
		FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id
		WHERE so.salesperson_id = $1
		  AND so.created_at >= date_trunc('month', CURRENT_DATE)
	`, salespersonID).Scan(&monthOrders, &monthAmount)

	// ── Today (unfiltered) ──
	var todayOrders int
	var todayAmount float64
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(si.net_amount),0)
		FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id
		WHERE so.salesperson_id = $1
		  AND so.created_at::date = CURRENT_DATE
	`, salespersonID).Scan(&todayOrders, &todayAmount)

	// ── Category-wise breakdown (respects date filter only) ──
	catWhere := ` WHERE so.salesperson_id = $1`
	catArgs := []interface{}{salespersonID}
	cn := 1
	if f.From != "" {
		cn++
		catWhere += fmt.Sprintf(` AND so.created_at >= $%d::date`, cn)
		catArgs = append(catArgs, f.From)
	}
	if f.To != "" {
		cn++
		catWhere += fmt.Sprintf(` AND so.created_at < ($%d::date + INTERVAL '1 day')`, cn)
		catArgs = append(catArgs, f.To)
	}

	catRows, err := s.db.Query(fmt.Sprintf(`
		SELECT COALESCE(cat.id::text, '') AS category_id,
		       COALESCE(cat.name, 'Uncategorized') AS category_name,
		       COUNT(DISTINCT so.id) AS order_count,
		       COALESCE(SUM(soi.total_price), 0) AS total_amount
		FROM sales_orders so
		JOIN sales_order_items soi ON soi.sales_order_id = so.id
		JOIN variants v ON v.id = soi.variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN categories cat ON cat.id = p.category_id
		%s
		GROUP BY cat.id, cat.name
		ORDER BY total_amount DESC
	`, catWhere), catArgs...)
	if err != nil {
		return nil, err
	}
	defer catRows.Close()

	var categories []map[string]interface{}
	for catRows.Next() {
		var catID, catName string
		var orderCount int
		var catAmount float64
		if err := catRows.Scan(&catID, &catName, &orderCount, &catAmount); err != nil {
			continue
		}
		categories = append(categories, map[string]interface{}{
			"category_id":   catID,
			"category_name": catName,
			"order_count":   orderCount,
			"total_amount":  catAmount,
		})
	}
	if categories == nil {
		categories = []map[string]interface{}{}
	}

	// ── Recent sales (respects all filters) ──
	recentArgs := make([]interface{}, len(args))
	copy(recentArgs, args)

	selectCols := `so.id, si.invoice_number, so.so_number, so.created_at, so.created_at::text AS created_at_text,
		       c.name AS customer_name, c.phone AS customer_phone,
		       si.net_amount, si.paid_amount, so.status, so.payment_status`
	recentJoin := `FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id
		JOIN customers c ON c.id = so.customer_id`
	if f.CategoryID != "" {
		recentJoin += `
		JOIN sales_order_items soi ON soi.sales_order_id = so.id
		JOIN variants v ON v.id = soi.variant_id
		JOIN products p ON p.id = v.product_id`
	}

	recentQ := fmt.Sprintf(`SELECT DISTINCT %s %s %s ORDER BY so.created_at DESC`,
		selectCols, recentJoin, where)

	salesRows, err := s.db.Query(recentQ, recentArgs...)
	if err != nil {
		return nil, err
	}
	defer salesRows.Close()

	var sales []map[string]interface{}
	for salesRows.Next() {
		var soID, invNum, soNum, soDate, custName, custPhone, st, paySt string
		var netAmt, paidAmt float64
		var createdAtRaw interface{}
		if err := salesRows.Scan(&soID, &invNum, &soNum, &createdAtRaw, &soDate, &custName, &custPhone, &netAmt, &paidAmt, &st, &paySt); err != nil {
			continue
		}
		sales = append(sales, map[string]interface{}{
			"sales_order_id": soID,
			"invoice_number": invNum,
			"so_number":      soNum,
			"date":           soDate,
			"customer_name":  custName,
			"customer_phone": custPhone,
			"net_amount":     netAmt,
			"paid_amount":    paidAmt,
			"balance_due":    netAmt - paidAmt,
			"status":         st,
			"payment_status": paySt,
		})
	}
	if sales == nil {
		sales = []map[string]interface{}{}
	}

	report := map[string]interface{}{
		"all_time_orders":    allOrders,
		"all_time_amount":    allAmount,
		"this_month_orders":  monthOrders,
		"this_month_amount":  monthAmount,
		"today_orders":       todayOrders,
		"today_amount":       todayAmount,
		"filtered_orders":    totalOrders,
		"filtered_amount":    totalAmount,
		"category_breakdown": categories,
		"recent_sales":       sales,
	}

	if f.From != "" || f.To != "" || f.CategoryID != "" {
		report["filters_applied"] = map[string]interface{}{
			"from":        f.From,
			"to":          f.To,
			"category_id": f.CategoryID,
		}
	}

	return report, nil
}

func (s *Store) Update(id string, in UpdateSalesPersonInput) error {
	_, err := s.db.Exec(`
		UPDATE sales_persons SET
			branch_id  = COALESCE($1, branch_id),
			name       = COALESCE($2, name),
			phone      = COALESCE($3, phone),
			email      = COALESCE($4, email)
		WHERE id = $5
	`, in.BranchID, in.Name, in.Phone, in.Email, id)
	return err
}

func (s *Store) SetActive(id string, active bool) error {
	res, err := s.db.Exec(`UPDATE sales_persons SET is_active = $1 WHERE id = $2`, active, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetBranchID returns the branch_id of a salesperson (for authorization checks).
func (s *Store) GetBranchID(id string) (*string, error) {
	var brID sql.NullString
	err := s.db.QueryRow(`SELECT branch_id FROM sales_persons WHERE id = $1`, id).Scan(&brID)
	if err != nil {
		return nil, err
	}
	if brID.Valid {
		return &brID.String, nil
	}
	return nil, nil
}

func nullStr(ns sql.NullString) interface{} {
	if ns.Valid {
		return ns.String
	}
	return nil
}
