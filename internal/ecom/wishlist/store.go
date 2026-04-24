package wishlist

import (
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Add adds a product to the customer's wishlist. Idempotent.
func (s *Store) Add(customerID, productID string) error {
	var exists bool
	s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM products WHERE id = $1 AND is_active = true)`, productID).Scan(&exists)
	if !exists {
		return fmt.Errorf("product not found or inactive")
	}

	_, err := s.db.Exec(`
		INSERT INTO ecom_wishlist_items (customer_id, product_id)
		VALUES ($1, $2)
		ON CONFLICT (customer_id, product_id) DO NOTHING
	`, customerID, productID)
	return err
}

// Remove removes a product from the customer's wishlist.
func (s *Store) Remove(customerID, productID string) error {
	res, err := s.db.Exec(`
		DELETE FROM ecom_wishlist_items WHERE customer_id = $1 AND product_id = $2
	`, customerID, productID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("item not in wishlist")
	}
	return nil
}

// List returns the customer's wishlist with product details and stock availability.
func (s *Store) List(customerID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT w.product_id, p.name AS product_name,
		       COALESCE(p.main_image_url, ''),
		       COALESCE(c.name, '') AS category,
		       EXISTS(
		           SELECT 1 FROM online_stocks os
		           JOIN variants v ON v.id = os.variant_id
		           WHERE v.product_id = w.product_id AND os.quantity > 0
		       ) AS in_stock,
		       w.created_at
		FROM ecom_wishlist_items w
		JOIN products p ON p.id = w.product_id
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE w.customer_id = $1
		ORDER BY w.created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var productID, productName, image, category string
		var inStock bool
		var createdAt time.Time

		rows.Scan(&productID, &productName, &image, &category, &inStock, &createdAt)

		items = append(items, map[string]interface{}{
			"product_id":   productID,
			"product_name": productName,
			"image":        image,
			"category":     category,
			"in_stock":     inStock,
			"added_at":     createdAt,
		})
	}
	return items, nil
}
