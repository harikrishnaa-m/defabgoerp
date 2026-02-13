package product

import (
	"database/sql"
	"strings"
)


type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

//
// CREATE
//

// func (s *Store) Create(in CreateProductInput) error {
// 	_, err := s.db.Exec(`
// 	INSERT INTO products
// 	(name, category_id, brand, image_url, is_web_visible, is_stitched, uom)
// 	VALUES ($1,$2,$3,$4,
// 	        COALESCE($5, TRUE),
// 	        COALESCE($6, FALSE),
// 	        COALESCE($7,'Unit'))
// 	`,
// 		in.Name,
// 		in.CategoryID,
// 		in.Brand,
// 		in.ImageURL,
// 		in.IsWebVisible,
// 		in.IsStitched,
// 		in.UOM,
// 	)
// 	return err
// }

//
// LIST ACTIVE + category join + pagination
//

func (s *Store) ListImages(productID string) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT id, image_url
		FROM product_images
		WHERE product_id = $1
		ORDER BY created_at
	`, productID)
}


// func (s *Store) ListImages(productID string) (*sql.Rows, error) {
// 	return s.db.Query(`
// 	SELECT image_url
// 	FROM product_images
// 	WHERE product_id=$1
// 	ORDER BY created_at
// 	`, productID)
// }

func (s *Store) List(limit, offset int) (*sql.Rows, error) {

	return s.db.Query(`
	SELECT
		p.id,
		p.name,
		p.brand,
		p.main_image_url,
		p.is_web_visible,
		p.is_stitched,
		p.uom,
		p.created_at,
		c.id,
		c.name
	FROM products p
	JOIN categories c ON c.id = p.category_id
	WHERE p.is_active = TRUE
	ORDER BY p.created_at DESC
	LIMIT $1 OFFSET $2
	`, limit, offset)
}


func (s *Store) CountActive() (int, error) {

	var n int

	err := s.db.QueryRow(`
	SELECT COUNT(*)
	FROM products
	WHERE is_active = TRUE
	`).Scan(&n)

	return n, err
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
	  p.main_image_url,
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
	  is_web_visible = COALESCE($4,is_web_visible),
	  is_stitched = COALESCE($5,is_stitched),
	  uom = COALESCE($6,uom),
	  updated_at = NOW()
	WHERE id=$7
	`,
		in.Name,
		in.CategoryID,
		in.Brand,
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


func (s *Store) CreateProduct(
	in CreateProductInput,
	mainImageURL string,
) (string, error) {

	var id string

	err := s.db.QueryRow(`
	INSERT INTO products
	(name, category_id, brand, main_image_url)
	VALUES ($1,$2,$3,$4)
	RETURNING id
	`,
		in.Name,
		in.CategoryID,
		in.Brand,
		mainImageURL,
	).Scan(&id)

	return id, err
}



func (s *Store) InsertProductImage(productID, url string) error {

	_, err := s.db.Exec(`
	INSERT INTO product_images (product_id, image_url)
	VALUES ($1,$2)
	`, productID, url)

	return err
}



func (s *Store) GetMainImage(productID string) (string, error) {
	var url string
	err := s.db.QueryRow(`
	SELECT main_image_url
	FROM products
	WHERE id=$1
	`, productID).Scan(&url)
	return url, err
}

func (s *Store) UpdateMainImage(productID, url string) error {
	_, err := s.db.Exec(`
	UPDATE products
	SET main_image_url=$1, updated_at=NOW()
	WHERE id=$2
	`, url, productID)
	return err
}


func (s *Store) GetProductImage(id string) (string, error) {
	var url string
	err := s.db.QueryRow(`
	SELECT image_url
	FROM product_images
	WHERE id=$1
	`, id).Scan(&url)
	return url, err
}


func (s *Store) DeleteProductImage(id string) error {
	_, err := s.db.Exec(`
	DELETE FROM product_images
	WHERE id=$1
	`, id)
	return err
}


func extractKey(url string) string {
	// assumes CDN/base url + key
	// https://cdn/.../products/abc.jpg → products/abc.jpg
	i := strings.Index(url, "/products/")
	if i == -1 {
		return ""
	}
	return url[i+1:]
}



