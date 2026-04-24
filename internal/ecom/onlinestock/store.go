package onlinestock

import (
	"database/sql"
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

// List returns all online stock entries with variant/product info.
func (s *Store) List() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT os.variant_id, v.variant_code, v.name AS variant_name, v.sku,
		       p.id AS product_id, p.name AS product_name,
		       os.quantity, os.updated_at
		FROM online_stocks os
		JOIN variants v ON v.id = os.variant_id
		JOIN products p ON p.id = v.product_id
		ORDER BY p.name, v.variant_code
	`)
	if err != nil {
		return nil, err
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
	return items, nil
}
