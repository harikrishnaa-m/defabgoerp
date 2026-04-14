package employee

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

func (s *Store) employeeRoleID() (uint, error) {
	var id uint
	err := s.db.QueryRow(`SELECT id FROM roles WHERE name = 'Employee'`).Scan(&id)
	return id, err
}

// ──────────────────────────────────────────
// Create
// ──────────────────────────────────────────

func (s *Store) Create(in CreateEmployeeInput, passwordHash string) (map[string]interface{}, error) {
	roleID, err := s.employeeRoleID()
	if err != nil {
		return nil, fmt.Errorf("employee role not found: %w", err)
	}

	var id, name, email string
	var branchID sql.NullString
	var isActive bool
	var createdAt sql.NullTime

	err = s.db.QueryRow(`
		INSERT INTO users (name, email, password_hash, role_id, branch_id, is_active)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		RETURNING id, name, email, branch_id, is_active, created_at
	`, in.Name, in.Email, passwordHash, roleID, in.BranchID).Scan(
		&id, &name, &email, &branchID, &isActive, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}

	result := map[string]interface{}{
		"id":         id,
		"name":       name,
		"email":      email,
		"branch_id":  branchID.String,
		"is_active":  isActive,
		"role":       "Employee",
		"created_at": createdAt.Time,
	}

	if branchID.Valid {
		var brName string
		if s.db.QueryRow(`SELECT name FROM branches WHERE id = $1`, branchID.String).Scan(&brName) == nil {
			result["branch_name"] = brName
		}
	}

	return result, nil
}

// ──────────────────────────────────────────
// List
// ──────────────────────────────────────────

func (s *Store) List(branchID *string, search string, limit, offset int) ([]map[string]interface{}, int, error) {
	roleID, err := s.employeeRoleID()
	if err != nil {
		return nil, 0, err
	}

	where := fmt.Sprintf("WHERE u.role_id = %d", roleID)
	args := []interface{}{}
	n := 0

	if branchID != nil && *branchID != "" {
		n++
		where += fmt.Sprintf(" AND u.branch_id = $%d", n)
		args = append(args, *branchID)
	}
	if search != "" {
		n++
		where += fmt.Sprintf(" AND (u.name ILIKE $%d OR u.email ILIKE $%d)", n, n)
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM users u %s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n++
	limitP := n
	n++
	offsetP := n
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT u.id, u.name, u.email, u.branch_id, COALESCE(b.name,'') AS branch_name,
		       u.is_active, u.created_at
		FROM users u
		LEFT JOIN branches b ON b.id = u.branch_id
		%s
		ORDER BY u.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, limitP, offsetP)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var id, name, email string
		var brID, brName sql.NullString
		var active bool
		var cat sql.NullTime
		if err := rows.Scan(&id, &name, &email, &brID, &brName, &active, &cat); err != nil {
			return nil, 0, err
		}
		list = append(list, map[string]interface{}{
			"id":          id,
			"name":        name,
			"email":       email,
			"branch_id":   brID.String,
			"branch_name": brName.String,
			"is_active":   active,
			"role":        "Employee",
			"created_at":  cat.Time,
		})
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return list, total, nil
}

// ──────────────────────────────────────────
// GetByID
// ──────────────────────────────────────────

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var name, email string
	var brID, brName sql.NullString
	var active bool
	var cat sql.NullTime

	err := s.db.QueryRow(`
		SELECT u.name, u.email, u.branch_id, COALESCE(b.name,'') AS branch_name,
		       u.is_active, u.created_at
		FROM users u
		LEFT JOIN branches b ON b.id = u.branch_id
		WHERE u.id = $1
	`, id).Scan(&name, &email, &brID, &brName, &active, &cat)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":          id,
		"name":        name,
		"email":       email,
		"branch_id":   brID.String,
		"branch_name": brName.String,
		"is_active":   active,
		"role":        "Employee",
		"created_at":  cat.Time,
	}

	// Recent attendance (last 30 records)
	attRows, err := s.db.Query(`
		SELECT id, date, punch_in, punch_out, total_hours, notes
		FROM attendance
		WHERE user_id = $1
		ORDER BY date DESC
		LIMIT 30
	`, id)
	if err == nil {
		defer attRows.Close()
		var attendance []map[string]interface{}
		var totalHoursAll float64
		var daysPresent int
		for attRows.Next() {
			var aid, date, notes string
			var punchIn sql.NullTime
			var punchOut sql.NullTime
			var hours float64
			if err := attRows.Scan(&aid, &date, &punchIn, &punchOut, &hours, &notes); err == nil {
				row := map[string]interface{}{
					"id":          aid,
					"date":        date,
					"punch_in":    punchIn.Time,
					"punch_out":   nil,
					"total_hours": hours,
					"notes":       notes,
				}
				if punchOut.Valid {
					row["punch_out"] = punchOut.Time
				}
				attendance = append(attendance, row)
				totalHoursAll += hours
				daysPresent++
			}
		}
		if attendance == nil {
			attendance = []map[string]interface{}{}
		}
		result["attendance"] = attendance
		result["total_hours"] = totalHoursAll
		result["days_present"] = daysPresent
	}

	return result, nil
}

// ──────────────────────────────────────────
// Update
// ──────────────────────────────────────────

func (s *Store) Update(id string, in UpdateEmployeeInput) error {
	_, err := s.db.Exec(`
		UPDATE users SET
		  name = COALESCE($1, name),
		  email = COALESCE($2, email),
		  branch_id = COALESCE($3, branch_id),
		  is_active = COALESCE($4, is_active)
		WHERE id = $5
	`, in.Name, in.Email, in.BranchID, in.IsActive, id)
	return err
}
