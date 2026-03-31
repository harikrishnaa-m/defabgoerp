package cart

import (
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// getOrCreateCart returns the cart ID for a customer, creating one if needed.
func (s *Store) getOrCreateCart(customerID string) (string, error) {
	var cartID string
	err := s.db.QueryRow(`SELECT id FROM ecom_carts WHERE customer_id = $1`, customerID).Scan(&cartID)
	if err == nil {
		return cartID, nil
	}
	err = s.db.QueryRow(`
		INSERT INTO ecom_carts (customer_id) VALUES ($1)
		ON CONFLICT (customer_id) DO UPDATE SET updated_at = NOW()
		RETURNING id
	`, customerID).Scan(&cartID)
	return cartID, err
}

// AddItem adds a variant to the cart or increments its quantity.
func (s *Store) AddItem(customerID, variantID string, quantity int) error {
	cartID, err := s.getOrCreateCart(customerID)
	if err != nil {
		return err
	}

	// Verify variant exists and is active
	var exists bool
	err = s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM variants WHERE id = $1 AND is_active = true)`, variantID).Scan(&exists)
	if err != nil || !exists {
		return fmt.Errorf("variant not found or inactive")
	}

	_, err = s.db.Exec(`
		INSERT INTO ecom_cart_items (cart_id, variant_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (cart_id, variant_id)
		DO UPDATE SET quantity = ecom_cart_items.quantity + $3
	`, cartID, variantID, quantity)

	if err == nil {
		s.db.Exec(`UPDATE ecom_carts SET updated_at = NOW() WHERE id = $1`, cartID)
	}
	return err
}

// UpdateItemQty sets the quantity for a specific cart item.
func (s *Store) UpdateItemQty(customerID, itemID string, quantity int) error {
	res, err := s.db.Exec(`
		UPDATE ecom_cart_items SET quantity = $3
		WHERE id = $1 AND cart_id = (SELECT id FROM ecom_carts WHERE customer_id = $2)
	`, itemID, customerID, quantity)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cart item not found")
	}
	return nil
}

// RemoveItem deletes a cart item.
func (s *Store) RemoveItem(customerID, itemID string) error {
	res, err := s.db.Exec(`
		DELETE FROM ecom_cart_items
		WHERE id = $1 AND cart_id = (SELECT id FROM ecom_carts WHERE customer_id = $2)
	`, itemID, customerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cart item not found")
	}
	return nil
}

// ClearCart removes all items from the customer's cart.
func (s *Store) ClearCart(customerID string) error {
	_, err := s.db.Exec(`
		DELETE FROM ecom_cart_items
		WHERE cart_id = (SELECT id FROM ecom_carts WHERE customer_id = $1)
	`, customerID)
	return err
}

// GetCart returns the customer's cart with full variant/product details.
func (s *Store) GetCart(customerID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT ci.id, ci.variant_id, v.variant_code, v.name AS variant_name, v.sku, v.price,
		       COALESCE(v.barcode, ''), p.id AS product_id, p.name AS product_name,
		       COALESCE(p.main_image_url, '') AS product_image,
		       ci.quantity,
		       COALESCE(SUM(st.quantity), 0) AS available_stock
		FROM ecom_cart_items ci
		JOIN ecom_carts c ON c.id = ci.cart_id
		JOIN variants v ON v.id = ci.variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN stocks st ON st.variant_id = v.id
		WHERE c.customer_id = $1
		GROUP BY ci.id, ci.variant_id, v.variant_code, v.name, v.sku, v.price, v.barcode,
		         p.id, p.name, p.main_image_url, ci.quantity
		ORDER BY ci.created_at
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var itemID, variantID, variantName, sku, barcode string
		var productID, productName, productImage string
		var variantCode, quantity int
		var price, availableStock float64

		if err := rows.Scan(&itemID, &variantID, &variantCode, &variantName, &sku, &price,
			&barcode, &productID, &productName, &productImage,
			&quantity, &availableStock); err != nil {
			return nil, err
		}

		items = append(items, map[string]interface{}{
			"id":              itemID,
			"variant_id":      variantID,
			"variant_code":    variantCode,
			"variant_name":    variantName,
			"sku":             sku,
			"price":           price,
			"barcode":         barcode,
			"product_id":      productID,
			"product_name":    productName,
			"product_image":   productImage,
			"quantity":        quantity,
			"line_total":      price * float64(quantity),
			"available_stock": availableStock,
			"in_stock":        availableStock >= float64(quantity),
		})
	}
	return items, nil
}

// CartSummary returns item count and total for the cart.
func (s *Store) CartSummary(customerID string) (int, float64, error) {
	var count int
	var total float64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(ci.quantity), 0),
		       COALESCE(SUM(ci.quantity * v.price), 0)
		FROM ecom_cart_items ci
		JOIN ecom_carts c ON c.id = ci.cart_id
		JOIN variants v ON v.id = ci.variant_id
		WHERE c.customer_id = $1
	`, customerID).Scan(&count, &total)
	return count, total, err
}
