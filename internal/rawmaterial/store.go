package rawmaterial

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

// ListByWarehouse returns all raw material stock for a warehouse.
func (s *Store) ListByWarehouse(warehouseID string, limit, offset int) ([]RawMaterialStockRow, error) {
	rows, err := s.db.Query(`
		SELECT rms.id, rms.item_name, rms.hsn_code, rms.unit,
		       rms.warehouse_id, w.name AS warehouse_name,
		       rms.quantity, rms.updated_at::text
		FROM raw_material_stocks rms
		JOIN warehouses w ON w.id = rms.warehouse_id
		WHERE rms.warehouse_id = $1
		ORDER BY rms.item_name
		LIMIT $2 OFFSET $3
	`, warehouseID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RawMaterialStockRow
	for rows.Next() {
		var r RawMaterialStockRow
		if err := rows.Scan(
			&r.ID, &r.ItemName, &r.HSNCode, &r.Unit,
			&r.WarehouseID, &r.WarehouseName,
			&r.Quantity, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// ListAll returns all raw material stock across all warehouses.
func (s *Store) ListAll(hsnCode, search string, limit, offset int) ([]RawMaterialStockRow, int, error) {
	where := ""
	args := []interface{}{}
	n := 0
	clauses := []string{}
	if hsnCode != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("rms.hsn_code = $%d", n))
		args = append(args, hsnCode)
	}
	if search != "" {
		n++
		clauses = append(clauses, fmt.Sprintf("rms.item_name ILIKE $%d", n))
		args = append(args, "%"+search+"%")
	}
	if len(clauses) > 0 {
		where = " WHERE " + clauses[0]
		for _, c := range clauses[1:] {
			where += " AND " + c
		}
	}

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM raw_material_stocks rms JOIN warehouses w ON w.id = rms.warehouse_id%s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n++
	q := fmt.Sprintf(`
		SELECT rms.id, rms.item_name, rms.hsn_code, rms.unit,
		       rms.warehouse_id, w.name AS warehouse_name,
		       rms.quantity, rms.updated_at::text
		FROM raw_material_stocks rms
		JOIN warehouses w ON w.id = rms.warehouse_id%s
		ORDER BY rms.item_name, w.name
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []RawMaterialStockRow
	for rows.Next() {
		var r RawMaterialStockRow
		if err := rows.Scan(
			&r.ID, &r.ItemName, &r.HSNCode, &r.Unit,
			&r.WarehouseID, &r.WarehouseName,
			&r.Quantity, &r.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	return list, total, nil
}

// ListByBranch returns raw material stock for warehouses belonging to a branch.
func (s *Store) ListByBranch(branchID, hsnCode, search string, limit, offset int) ([]RawMaterialStockRow, int, error) {
	args := []interface{}{branchID}
	n := 1
	where := " WHERE w.branch_id = $1"
	if hsnCode != "" {
		n++
		where += fmt.Sprintf(" AND rms.hsn_code = $%d", n)
		args = append(args, hsnCode)
	}
	if search != "" {
		n++
		where += fmt.Sprintf(" AND rms.item_name ILIKE $%d", n)
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM raw_material_stocks rms JOIN warehouses w ON w.id = rms.warehouse_id%s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n++
	q := fmt.Sprintf(`
		SELECT rms.id, rms.item_name, rms.hsn_code, rms.unit,
		       rms.warehouse_id, w.name AS warehouse_name,
		       rms.quantity, rms.updated_at::text
		FROM raw_material_stocks rms
		JOIN warehouses w ON w.id = rms.warehouse_id%s
		ORDER BY rms.item_name
		LIMIT $%d OFFSET $%d
	`, where, n, n+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []RawMaterialStockRow
	for rows.Next() {
		var r RawMaterialStockRow
		if err := rows.Scan(
			&r.ID, &r.ItemName, &r.HSNCode, &r.Unit,
			&r.WarehouseID, &r.WarehouseName,
			&r.Quantity, &r.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	return list, total, nil
}

// ListMovementsByBranch returns raw material movements for warehouses in a branch.
func (s *Store) ListMovementsByBranch(branchID string, limit, offset int) ([]RawMaterialMovementRow, error) {
	rows, err := s.db.Query(`
		SELECT rm.id, rm.item_name, rm.warehouse_id, w.name AS warehouse_name,
		       rm.quantity, rm.movement_type,
		       rm.goods_receipt_id, gr.grn_number,
		       rm.purchase_order_id, po.po_number,
		       rm.reference, rm.created_at::text
		FROM raw_material_movements rm
		JOIN warehouses w ON w.id = rm.warehouse_id
		LEFT JOIN goods_receipts gr ON gr.id = rm.goods_receipt_id
		LEFT JOIN purchase_orders po ON po.id = rm.purchase_order_id
		WHERE w.branch_id = $1
		ORDER BY rm.created_at DESC
		LIMIT $2 OFFSET $3
	`, branchID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RawMaterialMovementRow
	for rows.Next() {
		var r RawMaterialMovementRow
		if err := rows.Scan(
			&r.ID, &r.ItemName, &r.WarehouseID, &r.WarehouseName,
			&r.Quantity, &r.MovementType,
			&r.GoodsReceiptID, &r.GRNNumber,
			&r.PurchaseOrderID, &r.PONumber,
			&r.Reference, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// ListMovements returns raw material movements, filterable by stock_id or item_name+warehouse_id.
func (s *Store) ListMovements(itemName, warehouseID string, limit, offset int) ([]RawMaterialMovementRow, error) {
	rows, err := s.db.Query(`
		SELECT rm.id, rm.item_name, rm.warehouse_id, w.name AS warehouse_name,
		       rm.quantity, rm.movement_type,
		       rm.goods_receipt_id, gr.grn_number,
		       rm.purchase_order_id, po.po_number,
		       rm.reference, rm.created_at::text
		FROM raw_material_movements rm
		JOIN warehouses w ON w.id = rm.warehouse_id
		LEFT JOIN goods_receipts gr ON gr.id = rm.goods_receipt_id
		LEFT JOIN purchase_orders po ON po.id = rm.purchase_order_id
		WHERE rm.item_name = $1 AND rm.warehouse_id = $2
		ORDER BY rm.created_at DESC
		LIMIT $3 OFFSET $4
	`, itemName, warehouseID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RawMaterialMovementRow
	for rows.Next() {
		var r RawMaterialMovementRow
		if err := rows.Scan(
			&r.ID, &r.ItemName, &r.WarehouseID, &r.WarehouseName,
			&r.Quantity, &r.MovementType,
			&r.GoodsReceiptID, &r.GRNNumber,
			&r.PurchaseOrderID, &r.PONumber,
			&r.Reference, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// ListMovementsByStockID returns movements for a specific stock record.
func (s *Store) ListMovementsByStockID(stockID string, limit, offset int) ([]RawMaterialMovementRow, error) {
	// Look up item_name and warehouse_id from the stock record
	var itemName, warehouseID string
	err := s.db.QueryRow(`SELECT item_name, warehouse_id FROM raw_material_stocks WHERE id = $1`, stockID).Scan(&itemName, &warehouseID)
	if err != nil {
		return nil, fmt.Errorf("stock record not found")
	}
	return s.ListMovements(itemName, warehouseID, limit, offset)
}

// GetMovementByID returns a single raw material movement by ID.

// ListAllMovements returns all raw material movements across all warehouses.
func (s *Store) ListAllMovements(limit, offset int) ([]RawMaterialMovementRow, error) {
	rows, err := s.db.Query(`
		SELECT rm.id, rm.item_name, rm.warehouse_id, w.name AS warehouse_name,
		       rm.quantity, rm.movement_type,
		       rm.goods_receipt_id, gr.grn_number,
		       rm.purchase_order_id, po.po_number,
		       rm.reference, rm.created_at::text
		FROM raw_material_movements rm
		JOIN warehouses w ON w.id = rm.warehouse_id
		LEFT JOIN goods_receipts gr ON gr.id = rm.goods_receipt_id
		LEFT JOIN purchase_orders po ON po.id = rm.purchase_order_id
		ORDER BY rm.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RawMaterialMovementRow
	for rows.Next() {
		var r RawMaterialMovementRow
		if err := rows.Scan(
			&r.ID, &r.ItemName, &r.WarehouseID, &r.WarehouseName,
			&r.Quantity, &r.MovementType,
			&r.GoodsReceiptID, &r.GRNNumber,
			&r.PurchaseOrderID, &r.PONumber,
			&r.Reference, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// GetMovementByID returns a single raw material movement by ID.
func (s *Store) GetMovementByID(id string) (RawMaterialMovementRow, error) {
	var r RawMaterialMovementRow
	err := s.db.QueryRow(`
		SELECT rm.id, rm.item_name, rm.warehouse_id, w.name AS warehouse_name,
		       rm.quantity, rm.movement_type,
		       rm.goods_receipt_id, gr.grn_number,
		       rm.purchase_order_id, po.po_number,
		       rm.reference, rm.created_at::text
		FROM raw_material_movements rm
		JOIN warehouses w ON w.id = rm.warehouse_id
		LEFT JOIN goods_receipts gr ON gr.id = rm.goods_receipt_id
		LEFT JOIN purchase_orders po ON po.id = rm.purchase_order_id
		WHERE rm.id = $1
	`, id).Scan(
		&r.ID, &r.ItemName, &r.WarehouseID, &r.WarehouseName,
		&r.Quantity, &r.MovementType,
		&r.GoodsReceiptID, &r.GRNNumber,
		&r.PurchaseOrderID, &r.PONumber,
		&r.Reference, &r.CreatedAt,
	)
	return r, err
}

func (s *Store) AdjustStock(in AdjustStockInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Fetch item_name and warehouse_id from stock record
	var itemName, warehouseID string
	err = tx.QueryRow(`SELECT item_name, warehouse_id FROM raw_material_stocks WHERE id = $1`, in.StockID).Scan(&itemName, &warehouseID)
	if err != nil {
		return fmt.Errorf("stock record not found")
	}

	// Subtract quantity from stock
	result, err := tx.Exec(`
		UPDATE raw_material_stocks
		SET quantity = quantity - $1, updated_at = NOW()
		WHERE id = $2 AND quantity >= $1
	`, in.Quantity, in.StockID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("insufficient stock")
	}

	// Log movement
	_, err = tx.Exec(`
		INSERT INTO raw_material_movements
			(item_name, warehouse_id, quantity, movement_type, reference)
		VALUES ($1, $2, $3, $4, $5)
	`, itemName, warehouseID, in.Quantity, in.Type, in.Reference)
	if err != nil {
		return err
	}

	return tx.Commit()
}
