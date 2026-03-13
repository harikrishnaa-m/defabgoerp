package stock

import "database/sql"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}
func (s *Store) Create(in StockCreateInput) (string, error) {
	var id string
	err := s.db.QueryRow(
		`INSERT INTO stocks (variant_id, warehouse_id, quantity, updated_at)
			 VALUES ($1, $2, $3, NOW())
			 RETURNING id`,
		in.VariantID,
		in.WarehouseID,
		in.Quantity,
	).Scan(&id)
	return id, err
}

// Update stock record
func (s *Store) Update(id string, in StockUpdateInput) error {
	_, err := s.db.Exec(
		`UPDATE stocks SET variant_id = $1, warehouse_id = $2, quantity = $3, updated_at = NOW() WHERE id = $4`,
		in.VariantID,
		in.WarehouseID,
		in.Quantity,
		id,
	)
	return err
}

// Branch-wise stock
func (s *Store) ListByBranch(branchID string, limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			p.id,
			p.name AS product_name,
			v.id AS variant_id,
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

func (s *Store) GetAll(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			p.id, p.name,
			v.id, v.name, v.sku,
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

func (s *Store) GetByProduct(productID string) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			v.id, v.name, v.sku,
			SUM(s.quantity) AS total_qty
		FROM stocks s
		JOIN variants v ON v.id = s.variant_id
		WHERE v.product_id = $1
		GROUP BY v.id, v.name, v.sku
		ORDER BY v.name
	`, productID)
}

func (s *Store) GetMovements(limit, offset int) (*sql.Rows, error) {
	return s.db.Query(`
		SELECT
			sm.id,
			v.name,
			sm.movement_type,
			sm.quantity,
			fw.name AS from_wh,
			tw.name AS to_wh,
			sm.created_at
		FROM stock_movements sm
		JOIN variants v ON v.id = sm.variant_id
		LEFT JOIN warehouses fw ON fw.id = sm.from_warehouse_id
		LEFT JOIN warehouses tw ON tw.id = sm.to_warehouse_id
		ORDER BY sm.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
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
