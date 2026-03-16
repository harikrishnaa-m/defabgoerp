package supplier

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

// CREATE
func (s *Store) Create(in CreateSupplierInput) (string, string, error) {
	// Auto-generate supplier code: SUP001, SUP002, ...
	var maxCode sql.NullString
	s.db.QueryRow(`SELECT MAX(supplier_code) FROM suppliers WHERE supplier_code LIKE 'SUP%'`).Scan(&maxCode)

	next := 1
	if maxCode.Valid && len(maxCode.String) > 3 {
		fmt.Sscanf(maxCode.String[3:], "%d", &next)
		next++
	}
	code := fmt.Sprintf("SUP%03d", next)

	var id string
	err := s.db.QueryRow(`
		INSERT INTO suppliers
			(supplier_code, name, phone, email, address, gst_number)
		VALUES
			($1, $2, $3, $4, $5, $6)
		RETURNING id
	`,
		code,
		in.Name,
		in.Phone,
		in.Email,
		in.Address,
		in.GSTNumber,
	).Scan(&id)

	return id, code, err
}

// LIST (ACTIVE ONLY)
func (s *Store) List(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			id, supplier_code, name, phone, email, address,
			gst_number, is_active, created_at, updated_at
		FROM suppliers
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
}

// GET BY ID
func (s *Store) Get(id string) *sql.Row {
	return s.db.QueryRow(`
		SELECT
			id, supplier_code, name, phone, email, address,
			gst_number, is_active, created_at, updated_at
		FROM suppliers
		WHERE id = $1
	`, id)
}

// UPDATE
func (s *Store) Update(id string, in UpdateSupplierInput) error {
	_, err := s.db.Exec(`
		UPDATE suppliers SET
			name        = COALESCE($1, name),
			phone       = COALESCE($2, phone),
			email       = COALESCE($3, email),
			address     = COALESCE($4, address),
			gst_number  = COALESCE($5, gst_number),
			updated_at  = NOW()
		WHERE id = $6
	`,
		in.Name,
		in.Phone,
		in.Email,
		in.Address,
		in.GSTNumber,
		id,
	)

	return err
}

// ACTIVATE / DEACTIVATE (SOFT DELETE)
func (s *Store) SetActive(id string, active bool) error {
	_, err := s.db.Exec(`
		UPDATE suppliers
		SET is_active = $1, updated_at = NOW()
		WHERE id = $2
	`, active, id)

	return err
}
