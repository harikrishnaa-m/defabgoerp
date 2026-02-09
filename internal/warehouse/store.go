package warehouse

import "database/sql"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(in CreateWarehouseInput) error {
	_, err := s.db.Exec(`
		INSERT INTO warehouses (branch_id, name, type)
		VALUES ($1,$2,$3)
	`,
		in.BranchID,
		in.Name,
		in.Type,
	)
	return err
}

func (s *Store) List() (*sql.Rows, error) {
	return s.db.Query(`
		SELECT id, branch_id, name, type, created_at
		FROM warehouses
		ORDER BY created_at DESC
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
