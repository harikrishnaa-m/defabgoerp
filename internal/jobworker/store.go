package jobworker

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

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ──────────────────────────────────────────
// Create
// ──────────────────────────────────────────

func (s *Store) Create(in CreateJobOrderWorkerInput) (*JobOrderWorker, error) {
	w := &JobOrderWorker{}
	var role sql.NullString
	err := s.db.QueryRow(`
		INSERT INTO job_order_workers (name, phone, role)
		VALUES ($1, $2, $3)
		RETURNING id, name, phone, role, is_active, created_at, updated_at
	`, in.Name, in.Phone, nilIfEmpty(in.Role)).Scan(
		&w.ID, &w.Name, &w.Phone, &role, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create job order worker: %w", err)
	}
	w.Role = role.String
	return w, nil
}

// ──────────────────────────────────────────
// List
// ──────────────────────────────────────────

// List returns workers matching the given role filter (exact match only —
// untagged/general workers are not included unless role is empty), optional
// search on name/phone, and optional active-only filter.
func (s *Store) List(role, search string, activeOnly bool, limit, offset int) ([]JobOrderWorker, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	n := 0

	if role != "" {
		n++
		where += fmt.Sprintf(" AND role = $%d", n)
		args = append(args, role)
	}
	if search != "" {
		n++
		where += fmt.Sprintf(" AND (name ILIKE $%d OR phone ILIKE $%d)", n, n)
		args = append(args, "%"+search+"%")
	}
	if activeOnly {
		where += " AND is_active = TRUE"
	}

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM job_order_workers %s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	pagingClause := ""
	if limit > 0 {
		n++
		limitP := n
		n++
		offsetP := n
		args = append(args, limit, offset)
		pagingClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", limitP, offsetP)
	}

	q := fmt.Sprintf(`
		SELECT id, name, COALESCE(phone,''), role, is_active, created_at, updated_at
		FROM job_order_workers
		%s
		ORDER BY name ASC
		%s
	`, where, pagingClause)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []JobOrderWorker
	for rows.Next() {
		var w JobOrderWorker
		var role sql.NullString
		if err := rows.Scan(&w.ID, &w.Name, &w.Phone, &role, &w.IsActive, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, 0, err
		}
		w.Role = role.String
		list = append(list, w)
	}
	if list == nil {
		list = []JobOrderWorker{}
	}
	return list, total, nil
}

// ──────────────────────────────────────────
// GetByID
// ──────────────────────────────────────────

func (s *Store) GetByID(id string) (*JobOrderWorker, error) {
	w := &JobOrderWorker{}
	var role sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, COALESCE(phone,''), role, is_active, created_at, updated_at
		FROM job_order_workers WHERE id = $1
	`, id).Scan(&w.ID, &w.Name, &w.Phone, &role, &w.IsActive, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	w.Role = role.String
	return w, nil
}

// ──────────────────────────────────────────
// Update
// ──────────────────────────────────────────

func (s *Store) Update(id string, in UpdateJobOrderWorkerInput) error {
	q := `UPDATE job_order_workers SET updated_at = NOW()`
	args := []interface{}{}
	n := 0

	if in.Name != nil {
		n++
		q += fmt.Sprintf(", name = $%d", n)
		args = append(args, *in.Name)
	}
	if in.Phone != nil {
		n++
		q += fmt.Sprintf(", phone = $%d", n)
		args = append(args, *in.Phone)
	}
	if in.Role != nil {
		n++
		q += fmt.Sprintf(", role = $%d", n)
		args = append(args, nilIfEmpty(*in.Role))
	}
	if in.IsActive != nil {
		n++
		q += fmt.Sprintf(", is_active = $%d", n)
		args = append(args, *in.IsActive)
	}

	n++
	q += fmt.Sprintf(" WHERE id = $%d", n)
	args = append(args, id)

	res, err := s.db.Exec(q, args...)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
