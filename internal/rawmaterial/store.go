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
func (s *Store) ListAll(limit, offset int) ([]RawMaterialStockRow, error) {
	rows, err := s.db.Query(`
		SELECT rms.id, rms.item_name, rms.hsn_code, rms.unit,
		       rms.warehouse_id, w.name AS warehouse_name,
		       rms.quantity, rms.updated_at::text
		FROM raw_material_stocks rms
		JOIN warehouses w ON w.id = rms.warehouse_id
		ORDER BY rms.item_name, w.name
		LIMIT $1 OFFSET $2
	`, limit, offset)
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

// AdjustStock subtracts (OUT) or corrects (ADJUSTMENT) raw material quantity by stock ID.
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
