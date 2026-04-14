package salesperson

import (
	"database/sql"
	"fmt"

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

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
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
	salesReport, err := s.getSalesReport(id)
	if err == nil {
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

func (s *Store) getSalesReport(salespersonID string) (map[string]interface{}, error) {
	// Overall sales summary from sales_orders
	var totalOrders int
	var totalAmount float64
	err := s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(si.net_amount),0)
		FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id
		WHERE so.salesperson_id = $1
	`, salespersonID).Scan(&totalOrders, &totalAmount)
	if err != nil {
		return nil, err
	}

	// This month
	var monthOrders int
	var monthAmount float64
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(si.net_amount),0)
		FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id
		WHERE so.salesperson_id = $1
		  AND so.created_at >= date_trunc('month', CURRENT_DATE)
	`, salespersonID).Scan(&monthOrders, &monthAmount)

	// Today
	var todayOrders int
	var todayAmount float64
	s.db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(si.net_amount),0)
		FROM sales_orders so
		JOIN sales_invoices si ON si.sales_order_id = so.id
		WHERE so.salesperson_id = $1
		  AND so.created_at::date = CURRENT_DATE
	`, salespersonID).Scan(&todayOrders, &todayAmount)

	return map[string]interface{}{
		"total_orders":      totalOrders,
		"total_amount":      totalAmount,
		"this_month_orders": monthOrders,
		"this_month_amount": monthAmount,
		"today_orders":      todayOrders,
		"today_amount":      todayAmount,
	}, nil
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
