package role

import (
	"database/sql"
	"defab-erp/internal/core/model"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(name, permissions string) error {
	_, err := s.db.Exec(
		`INSERT INTO roles (name, permissions)
		 VALUES ($1, $2)`,
		name,
		permissions,
	)
	return err
}

func (s *Store) List() ([]model.Role, error) {
	rows, err := s.db.Query(
		`SELECT id, name, permissions
		 FROM roles
		 ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Role

	for rows.Next() {
		var r model.Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Permissions); err != nil {
			return nil, err
		}
		out = append(out, r)
	}

	return out, nil
}
