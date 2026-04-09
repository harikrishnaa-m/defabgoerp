package stock

import (
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create uses upsert — if variant+warehouse already exists, adds to quantity.
func (s *Store) Create(in StockCreateInput) (string, error) {
	var id string
	err := s.db.QueryRow(`
		INSERT INTO stocks (variant_id, warehouse_id, quantity, stock_type, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (variant_id, warehouse_id)
		DO UPDATE SET quantity = stocks.quantity + EXCLUDED.quantity,
		             stock_type = EXCLUDED.stock_type,
		             updated_at = NOW()
		RETURNING id`,
		in.VariantID,
		in.WarehouseID,
		in.Quantity,
		in.StockType,
	).Scan(&id)
	return id, err
}

// Update stock record (raw overwrite, kept for backward compat)
func (s *Store) Update(id string, in StockUpdateInput) error {
	_, err := s.db.Exec(
		`UPDATE stocks SET variant_id = $1, warehouse_id = $2, quantity = $3, stock_type = $4, updated_at = NOW() WHERE id = $5`,
		in.VariantID,
		in.WarehouseID,
		in.Quantity,
		in.StockType,
		id,
	)
	return err
}

// Adjust changes quantity and records a stock_movement for audit.
func (s *Store) Adjust(stockID string, newQty decimal.Decimal, reason, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var variantID, warehouseID string
	var oldQty decimal.Decimal
	err = tx.QueryRow(`
		SELECT variant_id, warehouse_id, quantity
		FROM stocks WHERE id = $1 FOR UPDATE
	`, stockID).Scan(&variantID, &warehouseID, &oldQty)
	if err != nil {
		return fmt.Errorf("stock not found: %w", err)
	}

	diff := newQty.Sub(oldQty)
	if diff.IsZero() {
		return nil // nothing to adjust
	}

	_, err = tx.Exec(`UPDATE stocks SET quantity = $1, updated_at = NOW() WHERE id = $2`, newQty, stockID)
	if err != nil {
		return err
	}

	// Record adjustment movement (positive = IN, negative = OUT)
	movementType := "ADJUSTMENT_IN"
	qty := diff
	if diff.IsNegative() {
		movementType = "ADJUSTMENT_OUT"
		qty = diff.Abs()
	}

	ref := reason
	if ref == "" {
		ref = "Manual adjustment"
	}

	_, err = tx.Exec(`
		INSERT INTO stock_movements
			(variant_id, from_warehouse_id, quantity, movement_type, reference, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'COMPLETED', NOW())
	`, variantID, warehouseID, qty, movementType, ref)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetByID returns a single stock record.
func (s *Store) GetByID(id string) (*sql.Row, error) {
	return s.db.QueryRow(`
		SELECT
			s.id, s.variant_id, v.variant_code, v.name AS variant_name, v.sku,
			p.id AS product_id, p.name AS product_name,
			s.warehouse_id, w.name AS warehouse_name,
			s.quantity, s.stock_type, s.updated_at
		FROM stocks s
		JOIN variants v ON v.id = s.variant_id
		JOIN products p ON p.id = v.product_id
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE s.id = $1
	`, id), nil
}

// Delete removes a stock record.
func (s *Store) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM stocks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("stock not found")
	}
	return nil
}

// Branch-wise stock
func (s *Store) ListByBranch(branchID string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			p.id,
			p.name AS product_name,
			v.id AS variant_id,
			v.variant_code,
			v.name AS variant_name,
			w.id AS warehouse_id,
			w.name AS warehouse_name,
			s.quantity
		FROM stocks s
		JOIN variants v ON v.id = s.variant_id
		JOIN products p ON p.id = v.product_id
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.branch_id = $1
		ORDER BY p.name, v.name
		LIMIT $2 OFFSET $3
	`, branchID, limit, offset)
}

// Warehouse-wise stock
func (s *Store) ListByWarehouse(warehouseID string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT
		v.id,
		v.variant_code,
		p.name AS product_name,
		v.name AS variant_name,
		w.name AS warehouse_name,
		s.quantity
	FROM stocks s
	JOIN variants v ON v.id = s.variant_id
	JOIN products p ON p.id = v.product_id
	JOIN warehouses w ON w.id = s.warehouse_id
	WHERE s.warehouse_id = $1
	ORDER BY p.name
	LIMIT $2 OFFSET $3
	`, warehouseID, limit, offset)
}

// Variant-wise stock (all warehouses)
func (s *Store) ListByVariant(variantID string) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT
		w.name AS warehouse,
		s.quantity
	FROM stocks s
	JOIN warehouses w ON w.id = s.warehouse_id
	WHERE s.variant_id = $1
	ORDER BY w.name
	`, variantID)
}

// Low stock (threshold)
func (s *Store) LowStock(threshold int) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT
		p.name AS product,
		v.name AS variant,
		v.variant_code,
		w.name AS warehouse,
		s.quantity
	FROM stocks s
	JOIN variants v ON v.id = s.variant_id
	JOIN products p ON p.id = v.product_id
	JOIN warehouses w ON w.id = s.warehouse_id
	WHERE s.quantity <= $1
	ORDER BY s.quantity ASC
	`, threshold)
}

func (s *Store) LowStockByBranch(threshold int, branchID string) (*sql.Rows, error) {
	return s.db.Query(`
	SELECT
		p.name AS product,
		v.name AS variant,
		v.variant_code,
		w.name AS warehouse,
		s.quantity
	FROM stocks s
	JOIN variants v ON v.id = s.variant_id
	JOIN products p ON p.id = v.product_id
	JOIN warehouses w ON w.id = s.warehouse_id
	WHERE s.quantity <= $1 AND w.branch_id = $2
	ORDER BY s.quantity ASC
	`, threshold, branchID)
}

func (s *Store) GetAll(variantCode string, limit, offset int) (*sql.Rows, error) {
	if variantCode != "" {
		return s.db.Query(`
			SELECT
				s.id,
				p.id, p.name,
				v.id, v.variant_code, v.name, v.sku,
				w.id, w.name,
				s.quantity
			FROM stocks s
			JOIN variants v   ON v.id = s.variant_id
			JOIN products p   ON p.id = v.product_id
			JOIN warehouses w ON w.id = s.warehouse_id
			WHERE v.variant_code::text = $1
			ORDER BY p.name, v.name
			LIMIT $2 OFFSET $3
		`, variantCode, limit, offset)
	}
	return s.db.Query(`
		SELECT
			s.id,
			p.id, p.name,
			v.id, v.variant_code, v.name, v.sku,
			w.id, w.name,
			s.quantity
		FROM stocks s
		JOIN variants v   ON v.id = s.variant_id
		JOIN products p   ON p.id = v.product_id
		JOIN warehouses w ON w.id = s.warehouse_id
		ORDER BY p.name, v.name
		LIMIT $1 OFFSET $2
	`, limit, offset)
}

func (s *Store) GetAllByBranch(branchID, variantCode string, limit, offset int) (*sql.Rows, error) {
	if variantCode != "" {
		return s.db.Query(`
			SELECT
				s.id,
				p.id, p.name,
				v.id, v.variant_code, v.name, v.sku,
				w.id, w.name,
				s.quantity
			FROM stocks s
			JOIN variants v   ON v.id = s.variant_id
			JOIN products p   ON p.id = v.product_id
			JOIN warehouses w ON w.id = s.warehouse_id
			WHERE w.branch_id = $1 AND v.variant_code::text = $2
			ORDER BY p.name, v.name
			LIMIT $3 OFFSET $4
		`, branchID, variantCode, limit, offset)
	}
	return s.db.Query(`
		SELECT
			s.id,
			p.id, p.name,
			v.id, v.variant_code, v.name, v.sku,
			w.id, w.name,
			s.quantity
		FROM stocks s
		JOIN variants v   ON v.id = s.variant_id
		JOIN products p   ON p.id = v.product_id
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.branch_id = $1
		ORDER BY p.name, v.name
		LIMIT $2 OFFSET $3
	`, branchID, limit, offset)
}

func (s *Store) CountAllByBranch(branchID, variantCode string) (int, error) {
	var total int
	if variantCode != "" {
		err := s.db.QueryRow(`
			SELECT COUNT(*) FROM stocks s
			JOIN variants v ON v.id = s.variant_id
			JOIN warehouses w ON w.id = s.warehouse_id
			WHERE w.branch_id = $1 AND v.variant_code::text = $2
		`, branchID, variantCode).Scan(&total)
		return total, err
	}
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM stocks s
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.branch_id = $1
	`, branchID).Scan(&total)
	return total, err
}

// GetAvailable returns stocks in the CENTRAL warehouse that are not in the given branch
func (s *Store) GetAvailable(branchID string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			s.id,
			p.id, p.name,
			v.id, v.variant_code, v.name, v.sku,
			w.id, w.name,
			s.quantity
		FROM stocks s
		JOIN variants v   ON v.id = s.variant_id
		JOIN products p   ON p.id = v.product_id
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.type = 'CENTRAL'
		  AND s.quantity > 0
		ORDER BY p.name, v.name
		LIMIT $1 OFFSET $2
	`, limit, offset)
}

func (s *Store) CountAvailable(branchID string) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM stocks s
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.type = 'CENTRAL'
		  AND s.quantity > 0
	`).Scan(&total)
	return total, err
}

// GetAvailableNew returns central stocks whose variant does NOT exist in any warehouse belonging to the branch
func (s *Store) GetAvailableNew(branchID string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			s.id,
			p.id, p.name,
			v.id, v.variant_code, v.name, v.sku,
			w.id, w.name,
			s.quantity
		FROM stocks s
		JOIN variants v   ON v.id = s.variant_id
		JOIN products p   ON p.id = v.product_id
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.type = 'CENTRAL'
		  AND s.quantity > 0
		  AND s.variant_id NOT IN (
		      SELECT s2.variant_id FROM stocks s2
		      JOIN warehouses w2 ON w2.id = s2.warehouse_id
		      WHERE w2.branch_id = $1 AND s2.quantity > 0
		  )
		ORDER BY p.name, v.name
		LIMIT $2 OFFSET $3
	`, branchID, limit, offset)
}

func (s *Store) CountAvailableNew(branchID string) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM stocks s
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.type = 'CENTRAL'
		  AND s.quantity > 0
		  AND s.variant_id NOT IN (
		      SELECT s2.variant_id FROM stocks s2
		      JOIN warehouses w2 ON w2.id = s2.warehouse_id
		      WHERE w2.branch_id = $1 AND s2.quantity > 0
		  )
	`, branchID).Scan(&total)
	return total, err
}

func (s *Store) GetByProduct(productID string) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			v.id, v.variant_code, v.name, v.sku,
			SUM(s.quantity) AS total_qty
		FROM stocks s
		JOIN variants v ON v.id = s.variant_id
		WHERE v.product_id = $1
		GROUP BY v.id, v.variant_code, v.name, v.sku
		ORDER BY v.name
	`, productID)
}

// GetMovements with optional filters: variant_id, warehouse_id, movement_type, from_date, to_date
func (s *Store) GetMovements(variantID, warehouseID, movementType, fromDate, toDate *string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			sm.id,
			v.id AS variant_id, v.variant_code, v.name AS variant_name,
			p.id AS product_id, p.name AS product_name,
			sm.movement_type,
			sm.quantity,
			sm.from_warehouse_id, COALESCE(fw.name,'') AS from_wh,
			sm.to_warehouse_id, COALESCE(tw.name,'') AS to_wh,
			COALESCE(sm.reference,'') AS reference,
			sm.status,
			sm.created_at
		FROM stock_movements sm
		JOIN variants v ON v.id = sm.variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN warehouses fw ON fw.id = sm.from_warehouse_id
		LEFT JOIN warehouses tw ON tw.id = sm.to_warehouse_id
		WHERE
			($1::text IS NULL OR sm.variant_id::text = $1)
			AND ($2::text IS NULL OR sm.from_warehouse_id::text = $2 OR sm.to_warehouse_id::text = $2)
			AND ($3::text IS NULL OR sm.movement_type = $3)
			AND ($4::date IS NULL OR sm.created_at::date >= $4::date)
			AND ($5::date IS NULL OR sm.created_at::date <= $5::date)
		ORDER BY sm.created_at DESC
		LIMIT $6 OFFSET $7
	`, variantID, warehouseID, movementType, fromDate, toDate, limit, offset)
}

// CountMovements with same filters
func (s *Store) CountMovements(variantID, warehouseID, movementType, fromDate, toDate *string) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM stock_movements sm
		WHERE
			($1::text IS NULL OR sm.variant_id::text = $1)
			AND ($2::text IS NULL OR sm.from_warehouse_id::text = $2 OR sm.to_warehouse_id::text = $2)
			AND ($3::text IS NULL OR sm.movement_type = $3)
			AND ($4::date IS NULL OR sm.created_at::date >= $4::date)
			AND ($5::date IS NULL OR sm.created_at::date <= $5::date)
	`, variantID, warehouseID, movementType, fromDate, toDate).Scan(&total)
	return total, err
}

// CountAll returns total stock records.
func (s *Store) CountAll(variantCode string) (int, error) {
	var total int
	if variantCode != "" {
		err := s.db.QueryRow(`
			SELECT COUNT(*) FROM stocks s
			JOIN variants v ON v.id = s.variant_id
			WHERE v.variant_code::text = $1
		`, variantCode).Scan(&total)
		return total, err
	}
	err := s.db.QueryRow(`SELECT COUNT(*) FROM stocks`).Scan(&total)
	return total, err
}

// GetMovementByID returns a single stock movement with full details.
func (s *Store) GetMovementByID(id string) *sql.Row {
	return s.db.QueryRow(`
		SELECT
			sm.id,
			v.id, v.variant_code, v.name, v.sku,
			p.id, p.name,
			sm.movement_type,
			sm.quantity,
			sm.from_warehouse_id, COALESCE(fw.name,''),
			sm.to_warehouse_id, COALESCE(tw.name,''),
			COALESCE(sm.reference,''),
			sm.status,
			COALESCE(sm.stock_request_id::text,''),
			COALESCE(sm.purchase_order_id::text,''),
			COALESCE(sm.supplier_id::text,''),
			COALESCE(sm.sale_order_id::text,''),
			sm.created_at,
			sm.updated_at
		FROM stock_movements sm
		JOIN variants v ON v.id = sm.variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN warehouses fw ON fw.id = sm.from_warehouse_id
		LEFT JOIN warehouses tw ON tw.id = sm.to_warehouse_id
		WHERE sm.id = $1
	`, id)
}

// GetMovementsByBranch returns movements where from/to warehouse belongs to the branch.
func (s *Store) GetMovementsByBranch(branchID string, movementType, fromDate, toDate *string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			sm.id,
			v.id, v.variant_code, v.name,
			p.id, p.name,
			sm.movement_type,
			sm.quantity,
			sm.from_warehouse_id, COALESCE(fw.name,''),
			sm.to_warehouse_id, COALESCE(tw.name,''),
			COALESCE(sm.reference,''),
			sm.status,
			sm.created_at
		FROM stock_movements sm
		JOIN variants v ON v.id = sm.variant_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN warehouses fw ON fw.id = sm.from_warehouse_id
		LEFT JOIN warehouses tw ON tw.id = sm.to_warehouse_id
		WHERE (
			fw.branch_id = $1 OR tw.branch_id = $1
		)
		AND ($2::text IS NULL OR sm.movement_type = $2)
		AND ($3::date IS NULL OR sm.created_at::date >= $3::date)
		AND ($4::date IS NULL OR sm.created_at::date <= $4::date)
		ORDER BY sm.created_at DESC
		LIMIT $5 OFFSET $6
	`, branchID, movementType, fromDate, toDate, limit, offset)
}

func (s *Store) CountMovementsByBranch(branchID string, movementType, fromDate, toDate *string) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM stock_movements sm
		LEFT JOIN warehouses fw ON fw.id = sm.from_warehouse_id
		LEFT JOIN warehouses tw ON tw.id = sm.to_warehouse_id
		WHERE (
			fw.branch_id = $1 OR tw.branch_id = $1
		)
		AND ($2::text IS NULL OR sm.movement_type = $2)
		AND ($3::date IS NULL OR sm.created_at::date >= $3::date)
		AND ($4::date IS NULL OR sm.created_at::date <= $4::date)
	`, branchID, movementType, fromDate, toDate).Scan(&total)
	return total, err
}

// CountByWarehouse returns total stock records for a warehouse.
func (s *Store) CountByWarehouse(warehouseID string) (int, error) {
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM stocks WHERE warehouse_id = $1`, warehouseID).Scan(&total)
	return total, err
}

// CountByBranch returns total stock records for a branch.
func (s *Store) CountByBranch(branchID string) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM stocks s
		JOIN warehouses w ON w.id = s.warehouse_id
		WHERE w.branch_id = $1
	`, branchID).Scan(&total)
	return total, err
}

func (s *Store) GetWarehouseProductSummary(warehouseID string) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			p.id,
			p.name,
			SUM(s.quantity) AS total_qty
		FROM stocks s
		JOIN variants v ON v.id = s.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE s.warehouse_id = $1
		GROUP BY p.id, p.name
		ORDER BY p.name
	`, warehouseID)
}
