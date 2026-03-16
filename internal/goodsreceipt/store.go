package goodsreceipt

import (
	"database/sql"
	"errors"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// generateGRNNumber creates the next GRN-XXX number.
func (s *Store) generateGRNNumber() (string, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM goods_receipts`).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("GRN-%03d", count+1), nil
}

// Create creates a goods receipt, upserts stock, records stock movements,
// and updates received_qty on the purchase order items.
func (s *Store) Create(in CreateGoodsReceiptInput, userID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if len(in.Items) == 0 {
		return "", errors.New("at least one item required")
	}

	grnNumber, err := s.generateGRNNumber()
	if err != nil {
		return "", err
	}

	// Fetch supplier_id and warehouse_id from the purchase order
	var supplierID, warehouseID string
	err = tx.QueryRow(`SELECT supplier_id, warehouse_id FROM purchase_orders WHERE id = $1`, in.PurchaseOrderID).Scan(&supplierID, &warehouseID)
	if err != nil {
		return "", fmt.Errorf("fetch purchase order: %w", err)
	}

	// Insert goods_receipts row
	var grnID string
	err = tx.QueryRow(`
		INSERT INTO goods_receipts
			(grn_number, purchase_order_id, supplier_id, warehouse_id, received_by, received_date, reference, status)
		VALUES ($1, $2, $3, $4, $5, NOW(), $6, 'COMPLETED')
		RETURNING id
	`, grnNumber, in.PurchaseOrderID, supplierID, warehouseID, userID, in.Reference).Scan(&grnID)
	if err != nil {
		return "", fmt.Errorf("insert goods_receipts: %w", err)
	}

	for _, item := range in.Items {
		if item.ReceivedQty <= 0 {
			return "", errors.New("received_qty must be greater than zero")
		}

		// Fetch ordered_qty from purchase order item
		var orderedQty float64
		err := tx.QueryRow(`SELECT quantity FROM purchase_order_items WHERE id = $1`, item.PurchaseOrderItemID).Scan(&orderedQty)
		if err != nil {
			return "", fmt.Errorf("fetch po item quantity: %w", err)
		}

		// Insert goods_receipt_items row
		_, err = tx.Exec(`
			INSERT INTO goods_receipt_items
				(goods_receipt_id, purchase_order_item_id, ordered_qty, received_qty)
			VALUES ($1, $2, $3, $4)
		`, grnID, item.PurchaseOrderItemID, orderedQty, item.ReceivedQty)
		if err != nil {
			return "", fmt.Errorf("insert goods_receipt_items: %w", err)
		}

		// Update received_qty on the purchase order item
		_, err = tx.Exec(`
			UPDATE purchase_order_items
			SET received_qty = received_qty + $1
			WHERE id = $2
		`, item.ReceivedQty, item.PurchaseOrderItemID)
		if err != nil {
			return "", fmt.Errorf("update received_qty: %w", err)
		}

		// Fetch PO item details for raw material stock
		var itemName, hsnCode, unit string
		err = tx.QueryRow(`
			SELECT item_name, COALESCE(hsn_code,''), unit
			FROM purchase_order_items WHERE id = $1
		`, item.PurchaseOrderItemID).Scan(&itemName, &hsnCode, &unit)
		if err != nil {
			return "", fmt.Errorf("fetch po item details: %w", err)
		}

		// Upsert raw_material_stocks
		_, err = tx.Exec(`
			INSERT INTO raw_material_stocks (item_name, hsn_code, unit, warehouse_id, quantity, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (item_name, warehouse_id)
			DO UPDATE SET quantity = raw_material_stocks.quantity + $5,
			             hsn_code = COALESCE(NULLIF($2,''), raw_material_stocks.hsn_code),
			             unit = $3,
			             updated_at = NOW()
		`, itemName, hsnCode, unit, warehouseID, item.ReceivedQty)
		if err != nil {
			return "", fmt.Errorf("upsert raw_material_stocks: %w", err)
		}

		// Insert raw_material_movements
		_, err = tx.Exec(`
			INSERT INTO raw_material_movements
				(item_name, warehouse_id, quantity, movement_type, goods_receipt_id, purchase_order_id, reference)
			VALUES ($1, $2, $3, 'IN', $4, $5, $6)
		`, itemName, warehouseID, item.ReceivedQty, grnID, in.PurchaseOrderID, in.Reference)
		if err != nil {
			return "", fmt.Errorf("insert raw_material_movement: %w", err)
		}
	}

	// Update PO status based on received quantities
	_, err = tx.Exec(`
		UPDATE purchase_orders
		SET status = CASE
			WHEN (SELECT COUNT(*) FROM purchase_order_items WHERE purchase_order_id = $1 AND received_qty < quantity) = 0
				THEN 'RECEIVED'
			WHEN (SELECT SUM(received_qty) FROM purchase_order_items WHERE purchase_order_id = $1) > 0
				THEN 'PARTIAL'
			ELSE status
		END
		WHERE id = $1
	`, in.PurchaseOrderID)
	if err != nil {
		return "", fmt.Errorf("update po status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return grnID, nil
}

// GetByID returns a goods receipt with its items.
func (s *Store) GetByID(id string) (*GoodsReceiptResponse, error) {
	var grn GoodsReceiptResponse
	err := s.db.QueryRow(`
		SELECT gr.id, gr.grn_number, gr.purchase_order_id, po.po_number,
		       gr.supplier_id, s.name AS supplier_name,
		       gr.warehouse_id, w.name AS warehouse_name,
		       gr.received_by, u.name AS received_by_name,
		       gr.received_date::text, gr.reference, gr.status, gr.created_at::text
		FROM goods_receipts gr
		JOIN purchase_orders po ON po.id = gr.purchase_order_id
		JOIN suppliers s ON s.id = gr.supplier_id
		JOIN warehouses w ON w.id = gr.warehouse_id
		JOIN users u ON u.id = gr.received_by
		WHERE gr.id = $1
	`, id).Scan(
		&grn.ID, &grn.GRNNumber, &grn.PurchaseOrderID, &grn.PONumber,
		&grn.SupplierID, &grn.SupplierName,
		&grn.WarehouseID, &grn.WarehouseName,
		&grn.ReceivedBy, &grn.ReceivedByName,
		&grn.ReceivedDate, &grn.Reference, &grn.Status, &grn.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT gri.id, gri.purchase_order_item_id, poi.item_name,
		       COALESCE(poi.hsn_code, ''), poi.unit,
		       gri.ordered_qty, gri.received_qty
		FROM goods_receipt_items gri
		JOIN purchase_order_items poi ON poi.id = gri.purchase_order_item_id
		WHERE gri.goods_receipt_id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item GoodsReceiptItemResponse
		if err := rows.Scan(&item.ID, &item.PurchaseOrderItemID, &item.ItemName,
			&item.HSNCode, &item.Unit, &item.OrderedQty, &item.ReceivedQty); err != nil {
			return nil, err
		}
		grn.Items = append(grn.Items, item)
	}

	return &grn, nil
}

// ListByPO returns all goods receipts for a purchase order.
func (s *Store) ListByPO(poID string) ([]GoodsReceiptResponse, error) {
	rows, err := s.db.Query(`
		SELECT gr.id, gr.grn_number, gr.purchase_order_id, po.po_number,
		       gr.supplier_id, s.name AS supplier_name,
		       gr.warehouse_id, w.name AS warehouse_name,
		       gr.received_by, u.name AS received_by_name,
		       gr.received_date::text, gr.reference, gr.status, gr.created_at::text
		FROM goods_receipts gr
		JOIN purchase_orders po ON po.id = gr.purchase_order_id
		JOIN suppliers s ON s.id = gr.supplier_id
		JOIN warehouses w ON w.id = gr.warehouse_id
		JOIN users u ON u.id = gr.received_by
		WHERE gr.purchase_order_id = $1
		ORDER BY gr.created_at DESC
	`, poID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GoodsReceiptResponse
	for rows.Next() {
		var grn GoodsReceiptResponse
		if err := rows.Scan(
			&grn.ID, &grn.GRNNumber, &grn.PurchaseOrderID, &grn.PONumber,
			&grn.SupplierID, &grn.SupplierName,
			&grn.WarehouseID, &grn.WarehouseName,
			&grn.ReceivedBy, &grn.ReceivedByName,
			&grn.ReceivedDate, &grn.Reference, &grn.Status, &grn.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, grn)
	}

	if err := s.populateItems(results); err != nil {
		return nil, err
	}
	return results, nil
}

// List returns all goods receipts, newest first.
func (s *Store) List() ([]GoodsReceiptResponse, error) {
	rows, err := s.db.Query(`
		SELECT gr.id, gr.grn_number, gr.purchase_order_id, po.po_number,
		       gr.supplier_id, s.name AS supplier_name,
		       gr.warehouse_id, w.name AS warehouse_name,
		       gr.received_by, u.name AS received_by_name,
		       gr.received_date::text, gr.reference, gr.status, gr.created_at::text
		FROM goods_receipts gr
		JOIN purchase_orders po ON po.id = gr.purchase_order_id
		JOIN suppliers s ON s.id = gr.supplier_id
		JOIN warehouses w ON w.id = gr.warehouse_id
		JOIN users u ON u.id = gr.received_by
		ORDER BY gr.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GoodsReceiptResponse
	for rows.Next() {
		var grn GoodsReceiptResponse
		if err := rows.Scan(
			&grn.ID, &grn.GRNNumber, &grn.PurchaseOrderID, &grn.PONumber,
			&grn.SupplierID, &grn.SupplierName,
			&grn.WarehouseID, &grn.WarehouseName,
			&grn.ReceivedBy, &grn.ReceivedByName,
			&grn.ReceivedDate, &grn.Reference, &grn.Status, &grn.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, grn)
	}

	if err := s.populateItems(results); err != nil {
		return nil, err
	}
	return results, nil
}

// populateItems fetches items for each GRN in the slice.
func (s *Store) populateItems(grns []GoodsReceiptResponse) error {
	for i := range grns {
		rows, err := s.db.Query(`
			SELECT gri.id, gri.purchase_order_item_id, poi.item_name,
			       COALESCE(poi.hsn_code, ''), poi.unit,
			       gri.ordered_qty, gri.received_qty
			FROM goods_receipt_items gri
			JOIN purchase_order_items poi ON poi.id = gri.purchase_order_item_id
			WHERE gri.goods_receipt_id = $1
		`, grns[i].ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var item GoodsReceiptItemResponse
			if err := rows.Scan(&item.ID, &item.PurchaseOrderItemID, &item.ItemName,
				&item.HSNCode, &item.Unit, &item.OrderedQty, &item.ReceivedQty); err != nil {
				rows.Close()
				return err
			}
			grns[i].Items = append(grns[i].Items, item)
		}
		rows.Close()
	}
	return nil
}

// Cancel reverses a GRN: subtracts raw material stock, logs OUT movements,
// reverses PO item received_qty, recalculates PO status, marks GRN as CANCELLED.
func (s *Store) Cancel(grnID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check current status
	var status, poID, warehouseID string
	err = tx.QueryRow(`SELECT status, purchase_order_id, warehouse_id FROM goods_receipts WHERE id = $1`, grnID).Scan(&status, &poID, &warehouseID)
	if err != nil {
		return fmt.Errorf("fetch grn: %w", err)
	}
	if status == "CANCELLED" {
		return errors.New("GRN is already cancelled")
	}

	// Get GRN items with PO item details
	rows, err := tx.Query(`
		SELECT gri.purchase_order_item_id, gri.received_qty, poi.item_name
		FROM goods_receipt_items gri
		JOIN purchase_order_items poi ON poi.id = gri.purchase_order_item_id
		WHERE gri.goods_receipt_id = $1
	`, grnID)
	if err != nil {
		return fmt.Errorf("fetch grn items: %w", err)
	}
	defer rows.Close()

	type grnItem struct {
		poItemID    string
		receivedQty float64
		itemName    string
	}
	var items []grnItem
	for rows.Next() {
		var it grnItem
		if err := rows.Scan(&it.poItemID, &it.receivedQty, &it.itemName); err != nil {
			return err
		}
		items = append(items, it)
	}
	rows.Close()

	for _, it := range items {
		// Reverse PO item received_qty
		_, err = tx.Exec(`UPDATE purchase_order_items SET received_qty = received_qty - $1 WHERE id = $2`, it.receivedQty, it.poItemID)
		if err != nil {
			return fmt.Errorf("reverse po item qty: %w", err)
		}

		// Subtract raw material stock
		_, err = tx.Exec(`
			UPDATE raw_material_stocks
			SET quantity = quantity - $1, updated_at = NOW()
			WHERE item_name = $2 AND warehouse_id = $3
		`, it.receivedQty, it.itemName, warehouseID)
		if err != nil {
			return fmt.Errorf("reverse raw material stock: %w", err)
		}

		// Log OUT movement
		_, err = tx.Exec(`
			INSERT INTO raw_material_movements
				(item_name, warehouse_id, quantity, movement_type, goods_receipt_id, purchase_order_id, reference)
			VALUES ($1, $2, $3, 'OUT', $4, $5, 'GRN_CANCELLED')
		`, it.itemName, warehouseID, it.receivedQty, grnID, poID)
		if err != nil {
			return fmt.Errorf("insert reversal movement: %w", err)
		}
	}

	// Mark GRN as CANCELLED
	_, err = tx.Exec(`UPDATE goods_receipts SET status = 'CANCELLED' WHERE id = $1`, grnID)
	if err != nil {
		return fmt.Errorf("cancel grn: %w", err)
	}

	// Recalculate PO status
	_, err = tx.Exec(`
		UPDATE purchase_orders
		SET status = CASE
			WHEN (SELECT COALESCE(SUM(received_qty),0) FROM purchase_order_items WHERE purchase_order_id = $1) = 0
				THEN 'APPROVED'
			WHEN (SELECT COUNT(*) FROM purchase_order_items WHERE purchase_order_id = $1 AND received_qty < quantity) = 0
				THEN 'RECEIVED'
			ELSE 'PARTIAL'
		END
		WHERE id = $1
	`, poID)
	if err != nil {
		return fmt.Errorf("recalc po status: %w", err)
	}

	return tx.Commit()
}
