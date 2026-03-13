package warehouse

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

func (s *Store) Create(in CreateWarehouseInput) error {
	_, err := s.db.Exec(`
		INSERT INTO warehouses (branch_id, name, type, warehouse_code)
		VALUES ($1,$2,$3,$4)
	`,
		in.BranchID,
		in.Name,
		in.Type,
		in.WarehouseCode,
	)
	return err
}

// Generate next warehouse code (WHxxx)
func (s *Store) NextWarehouseCode() (string, error) {
	var maxCode sql.NullString
	err := s.db.QueryRow(`SELECT MAX(warehouse_code) FROM warehouses WHERE warehouse_code LIKE 'WH%'`).Scan(&maxCode)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return "", err
	}
	nextNum := 1
	if maxCode.Valid {
		var num int
		_, err := fmt.Sscanf(maxCode.String, "WH%03d", &num)
		if err == nil {
			nextNum = num + 1
		}
	}
	return fmt.Sprintf("WH%03d", nextNum), nil
}

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM warehouses WHERE id = $1", id)
	return err
}

func (s *Store) List() (*sql.Rows, error) {
	return s.db.Query(`
		SELECT w.id, w.branch_id, b.name as branch_name, w.name, w.type, w.created_at, w.warehouse_code,
			   b.city, b.state, b.phone_number
		FROM warehouses w
		LEFT JOIN branches b ON w.branch_id = b.id
		ORDER BY w.created_at DESC
	`)
}

func (s *Store) Update(id string, in UpdateWarehouseInput) error {
	_, err := s.db.Exec(`
		UPDATE warehouses
		SET
		  branch_id = COALESCE($1, branch_id),
		  name = COALESCE($2, name),
		  type = COALESCE($3, type)
		WHERE id = $4
	`,
		in.BranchID,
		in.Name,
		in.Type,
		id,
	)

	return err
}

func (s *Store) GetByID(id string) *sql.Row {
	return s.db.QueryRow(`
		SELECT w.id, w.branch_id, b.name as branch_name, w.name, w.type, w.created_at, w.warehouse_code,
			   b.city, b.state, b.phone_number
		FROM warehouses w
		LEFT JOIN branches b ON w.branch_id = b.id
		WHERE w.id = $1
	`, id)
}
