package category

import "database/sql"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

//
// CREATE
//

func (s *Store) Create(name string) error {
	_, err := s.db.Exec(
		`INSERT INTO categories (name) VALUES ($1)`,
		name,
	)
	return err
}

//
// LIST ACTIVE ONLY + pagination
//

func (s *Store) ListActive(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT id, name, is_active
		FROM categories
		WHERE is_active = TRUE
		ORDER BY name
		LIMIT $1 OFFSET $2
	`, limit, offset)
}

func (s *Store) CountActive() (int, error) {
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM categories WHERE is_active = TRUE`,
	).Scan(&total)
	return total, err
}

//
// GET BY ID (admin can see inactive too)
//

func (s *Store) Get(id string) (string, string, bool, error) {
	var cid, name string
	var active bool

	err := s.db.QueryRow(
		`SELECT id, name, is_active FROM categories WHERE id=$1`,
		id,
	).Scan(&cid, &name, &active)

	return cid, name, active, err
}

//
// UPDATE
//

func (s *Store) Update(id string, in UpdateCategoryInput) error {
	_, err := s.db.Exec(`
		UPDATE categories
		SET name = COALESCE($1, name)
		WHERE id = $2
	`, in.Name, id)

	return err
}

//
// SOFT DELETE / ACTIVATE
//

func (s *Store) SetActive(id string, active bool) error {
	_, err := s.db.Exec(
		`UPDATE categories SET is_active=$1 WHERE id=$2`,
		active, id,
	)
	return err
}
