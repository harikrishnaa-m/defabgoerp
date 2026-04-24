package onlinestock

import (
	"database/sql"
	"strconv"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Upsert sets (or replaces) the reserved online quantity for a variant.
func (s *Store) Upsert(variantID string, quantity int) error {
	_, err := s.db.Exec(`
		INSERT INTO online_stocks (variant_id, quantity)
		VALUES ($1, $2)
		ON CONFLICT (variant_id)
		DO UPDATE SET quantity = $2, updated_at = NOW()
	`, variantID, quantity)
	return err
}

// List returns all online stock entries with optional search and pagination.
func (s *Store) List(search string, page, limit int) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * limit

	where := ""
	args := []interface{}{}
	idx := 1

	if search != "" {
		where = ` WHERE (v.sku ILIKE $1 OR v.name ILIKE $1 OR p.name ILIKE $1 OR v.variant_code::text ILIKE $1)`
		args = append(args, "%"+search+"%")
		idx = 2
	}

	var total int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM online_stocks os
		JOIN variants v ON v.id = os.variant_id
		JOIN products p ON p.id = v.product_id`+where, args...).Scan(&total)

	args = append(args, limit, offset)
	rows, err := s.db.Query(`
		SELECT os.variant_id, v.variant_code, v.name AS variant_name, v.sku,
		       p.id AS product_id, p.name AS product_name,
		       os.quantity, os.updated_at
		FROM online_stocks os
		JOIN variants v ON v.id = os.variant_id
		JOIN products p ON p.id = v.product_id`+where+`
		ORDER BY p.name, v.variant_code
		LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var variantID, variantName, sku, productID, productName string
		var variantCode, quantity int
		var updatedAt time.Time

		rows.Scan(&variantID, &variantCode, &variantName, &sku,
			&productID, &productName, &quantity, &updatedAt)

		items = append(items, map[string]interface{}{
			"variant_id":   variantID,
			"variant_code": variantCode,
			"variant_name": variantName,
			"sku":          sku,
			"product_id":   productID,
			"product_name": productName,
			"quantity":     quantity,
			"updated_at":   updatedAt,
		})
	}
	return items, total, nil
}

// SyncAndListWebVisible inserts any missing variants (of is_web_visible products)
// into online_stocks at quantity=0, then returns the full list.
func (s *Store) SyncAndListWebVisible() ([]map[string]interface{}, int, error) {
	// Insert missing variants with quantity 0
	res, err := s.db.Exec(`
		INSERT INTO online_stocks (variant_id, quantity)
		SELECT v.id, 0
		FROM variants v
		JOIN products p ON p.id = v.product_id
		WHERE p.is_web_visible = true
		  AND v.is_active = true
		  AND NOT EXISTS (SELECT 1 FROM online_stocks os WHERE os.variant_id = v.id)
		ON CONFLICT (variant_id) DO NOTHING
	`)
	if err != nil {
		return nil, 0, err
	}
	newlyAdded, _ := res.RowsAffected()

	rows, err := s.db.Query(`
		SELECT os.variant_id, v.variant_code, v.name AS variant_name, v.sku,
		       p.id AS product_id, p.name AS product_name,
		       os.quantity, os.updated_at
		FROM online_stocks os
		JOIN variants v ON v.id = os.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE p.is_web_visible = true AND v.is_active = true
		ORDER BY p.name, v.variant_code
	`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var variantID, variantName, sku, productID, productName string
		var variantCode, quantity int
		var updatedAt time.Time

		rows.Scan(&variantID, &variantCode, &variantName, &sku,
			&productID, &productName, &quantity, &updatedAt)

		items = append(items, map[string]interface{}{
			"variant_id":   variantID,
			"variant_code": variantCode,
			"variant_name": variantName,
			"sku":          sku,
			"product_id":   productID,
			"product_name": productName,
			"quantity":     quantity,
			"updated_at":   updatedAt,
		})
	}
	return items, int(newlyAdded), nil
}

// ListLowStock returns online_stocks entries with quantity < 10, with optional search and pagination.
func (s *Store) ListLowStock(search string, page, limit int) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * limit

	where := " WHERE os.quantity < 10"
	args := []interface{}{}
	idx := 1

	if search != "" {
		where += ` AND (v.sku ILIKE $1 OR v.name ILIKE $1 OR p.name ILIKE $1 OR v.variant_code::text ILIKE $1)`
		args = append(args, "%"+search+"%")
		idx = 2
	}

	var total int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM online_stocks os
		JOIN variants v ON v.id = os.variant_id
		JOIN products p ON p.id = v.product_id`+where, args...).Scan(&total)

	args = append(args, limit, offset)
	rows, err := s.db.Query(`
		SELECT os.variant_id, v.variant_code, v.name AS variant_name, v.sku,
		       p.id AS product_id, p.name AS product_name,
		       os.quantity, os.updated_at
		FROM online_stocks os
		JOIN variants v ON v.id = os.variant_id
		JOIN products p ON p.id = v.product_id`+where+`
		ORDER BY os.quantity ASC, p.name
		LIMIT $`+itoa(idx)+` OFFSET $`+itoa(idx+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var variantID, variantName, sku, productID, productName string
		var variantCode, quantity int
		var updatedAt time.Time

		rows.Scan(&variantID, &variantCode, &variantName, &sku,
			&productID, &productName, &quantity, &updatedAt)

		items = append(items, map[string]interface{}{
			"variant_id":   variantID,
			"variant_code": variantCode,
			"variant_name": variantName,
			"sku":          sku,
			"product_id":   productID,
			"product_name": productName,
			"quantity":     quantity,
			"updated_at":   updatedAt,
		})
	}
	return items, total, nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
