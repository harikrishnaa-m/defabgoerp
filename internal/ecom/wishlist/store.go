package wishlist

import (
	"database/sql"
	"fmt"
	"strings"
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

// List returns the customer's wishlist with product details, stock availability, and all variants.
func (s *Store) List(customerID string) ([]map[string]interface{}, error) {
	// 1. Fetch wishlist products
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
	var productIDs []string

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
			"variants":     []map[string]interface{}{},
		})
		productIDs = append(productIDs, productID)
	}
	rows.Close()

	if len(productIDs) == 0 {
		return items, nil
	}

	// 2. Fetch all active variants for those products with online stock
	placeholders := make([]string, len(productIDs))
	args := make([]interface{}, len(productIDs))
	for i, pid := range productIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = pid
	}

	vRows, err := s.db.Query(`
		SELECT v.id, v.product_id, v.variant_code, v.name AS variant_name,
		       v.sku, v.price, COALESCE(v.barcode, ''),
		       COALESCE(os.quantity, 0) AS online_stock
		FROM variants v
		LEFT JOIN online_stocks os ON os.variant_id = v.id
		WHERE v.product_id IN (`+strings.Join(placeholders, ",")+`) AND v.is_active = true
		ORDER BY v.variant_code
	`, args...)
	if err != nil {
		return items, nil
	}
	defer vRows.Close()

	// Build map of productID -> variants
	variantMap := map[string][]map[string]interface{}{}
	for vRows.Next() {
		var variantID, productID, variantName, sku, barcode string
		var variantCode, onlineStock int
		var price float64

		vRows.Scan(&variantID, &productID, &variantCode, &variantName,
			&sku, &price, &barcode, &onlineStock)

		variantMap[productID] = append(variantMap[productID], map[string]interface{}{
			"variant_id":   variantID,
			"variant_code": variantCode,
			"variant_name": variantName,
			"sku":          sku,
			"price":        price,
			"barcode":      barcode,
			"online_stock": onlineStock,
			"in_stock":     onlineStock > 0,
		})
	}

	// 3. Attach variants to each wishlist item
	for i, item := range items {
		pid := item["product_id"].(string)
		if variants, ok := variantMap[pid]; ok {
			items[i]["variants"] = variants
		}
	}

	return items, nil
}
