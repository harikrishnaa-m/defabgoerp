package branch

import (
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(name, address, managerID string) error {
	_, err := s.db.Exec(
		`INSERT INTO branches (name, address, manager_id)
		 VALUES ($1,$2, NULLIF($3,'')::uuid)`,
		name, address, managerID,
	)
	return err
}

func (s *Store) List() (*sql.Rows, error) {
	return s.db.Query(
		`SELECT id, name, address, manager_id, created_at
		 FROM branches
		 ORDER BY id`,
	)
}


func (s *Store) Update(id int, in UpdateBranchInput) error {
	_, err := s.db.Exec(`
		UPDATE branches
		SET
		  name = COALESCE($1, name),
		  address = COALESCE($2, address),
		  manager_id = COALESCE(NULLIF($3,'')::uuid, manager_id)
		WHERE id = $4
	`,
		in.Name,
		in.Address,
		in.ManagerID,
		id,
	)

	return err
}
