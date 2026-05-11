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

func (s *Store) ListActive(search string) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT id, name, is_active, products_count, COALESCE(image_url, '')
		FROM categories
		WHERE name ILIKE $1
		ORDER BY name
	`, "%"+search+"%")
}

func (s *Store) ListActivePaged(search string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT id, name, is_active, products_count, COALESCE(image_url, '')
		FROM categories
		WHERE name ILIKE $1
		ORDER BY name
		LIMIT $2 OFFSET $3
	`, "%"+search+"%", limit, offset)
}

func (s *Store) CountActive(search string) (int, error) {
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM categories WHERE is_active = TRUE AND name ILIKE $1`,
		"%"+search+"%",
	).Scan(&total)
	return total, err
}

func (s *Store) ListProductsByCategory(categoryID, search string) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT p.id, p.name, COALESCE(p.brand, ''), COALESCE(p.main_image_url, ''), p.is_active
		FROM products p
		WHERE p.category_id = $1
		  AND p.name ILIKE $2
		ORDER BY p.name
	`, categoryID, "%"+search+"%")
}

func (s *Store) ListProductsByCategoryPaged(categoryID, search string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT p.id, p.name, COALESCE(p.brand, ''), COALESCE(p.main_image_url, ''), p.is_active
		FROM products p
		WHERE p.category_id = $1
		  AND p.name ILIKE $2
		ORDER BY p.name
		LIMIT $3 OFFSET $4
	`, categoryID, "%"+search+"%", limit, offset)
}

func (s *Store) CountProductsByCategory(categoryID, search string) (int, error) {
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM products WHERE category_id = $1 AND name ILIKE $2`,
		categoryID, "%"+search+"%",
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
