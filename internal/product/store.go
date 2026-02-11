package product

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

func (s *Store) Create(in CreateProductInput) error {
	_, err := s.db.Exec(`
	INSERT INTO products
	(name, category_id, brand, image_url, is_web_visible, is_stitched, uom)
	VALUES ($1,$2,$3,$4,
	        COALESCE($5, TRUE),
	        COALESCE($6, FALSE),
	        COALESCE($7,'Unit'))
	`,
		in.Name,
		in.CategoryID,
		in.Brand,
		in.ImageURL,
		in.IsWebVisible,
		in.IsStitched,
		in.UOM,
	)
	return err
}

//
// LIST ACTIVE + category join + pagination
//

func (s *Store) List(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT
	  p.id,
	  p.name,
	  p.brand,
	  p.image_url,
	  p.is_web_visible,
	  p.is_stitched,
	  p.uom,
	  p.created_at,
	  c.id,
	  c.name
	FROM products p
	JOIN categories c ON p.category_id = c.id
	WHERE p.is_active = TRUE
	ORDER BY p.created_at DESC
	LIMIT $1 OFFSET $2
	`, limit, offset)
}

func (s *Store) CountActive() (int, error) {
	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM products WHERE is_active=TRUE`,
	).Scan(&total)
	return total, err
}

//
// GET BY ID
//

func (s *Store) Get(id string) (*sql.Row) {
	return s.db.QueryRow(`
	SELECT
	  p.id,
	  p.name,
	  p.brand,
	  p.image_url,
	  p.is_web_visible,
	  p.is_stitched,
	  p.uom,
	  p.is_active,
	  c.id,
	  c.name
	FROM products p
	JOIN categories c ON p.category_id = c.id
	WHERE p.id=$1
	`, id)
}

//
// UPDATE
//

func (s *Store) Update(id string, in UpdateProductInput) error {
	_, err := s.db.Exec(`
	UPDATE products SET
	  name = COALESCE($1,name),
	  category_id = COALESCE($2,category_id),
	  brand = COALESCE($3,brand),
	  image_url = COALESCE($4,image_url),
	  is_web_visible = COALESCE($5,is_web_visible),
	  is_stitched = COALESCE($6,is_stitched),
	  uom = COALESCE($7,uom)
	WHERE id=$8
	`,
		in.Name,
		in.CategoryID,
		in.Brand,
		in.ImageURL,
		in.IsWebVisible,
		in.IsStitched,
		in.UOM,
		id,
	)
	return err
}

//
// SOFT DELETE / RESTORE
//

func (s *Store) SetActive(id string, active bool) error {
	_, err := s.db.Exec(
		`UPDATE products SET is_active=$1 WHERE id=$2`,
		active, id,
	)
	return err
}
