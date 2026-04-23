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

func (s *Store) Create(in CreateCategoryInput) error {
	_, err := s.db.Exec(
		`INSERT INTO categories (name, image_url) VALUES ($1, $2)`,
		in.Name, in.ImageURL,
	)
	return err
}

//
// LIST ACTIVE ONLY + pagination
//

func (s *Store) ListActive(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT id, name, is_active, products_count, COALESCE(image_url, '')
		FROM categories
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

func (s *Store) Get(id string) (string, string, bool, int, string, error) {
	var cid, name, imageURL string
	var active bool
	var productsCount int

	err := s.db.QueryRow(
		`SELECT id, name, is_active, products_count, COALESCE(image_url, '') FROM categories WHERE id=$1`,
		id,
	).Scan(&cid, &name, &active, &productsCount, &imageURL)

	return cid, name, active, productsCount, imageURL, err
}

//
// UPDATE
//

func (s *Store) Update(id string, in UpdateCategoryInput) error {
	_, err := s.db.Exec(`
		UPDATE categories
		SET name = COALESCE($1, name),
		    image_url = COALESCE($2, image_url)
		WHERE id = $3
	`, in.Name, in.ImageURL, id)

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
